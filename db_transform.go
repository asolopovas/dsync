package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

const (
	DBReplaceEngineRaw          = "raw"
	DBReplaceEngineGoSerialized = "go-serialized"
	DBReplaceEngineNone         = "none"
)

type ReplacementOptions struct {
	Engine             string
	ValidateSerialized bool
	Replacements       []DBReplace
	SkipColumns        []string
	compiled           compiledReplacements
	skipped            map[string]struct{}
}

type sqlValue struct {
	Raw      string
	IsString bool
	String   string
}

func ReplacementOptionsFromConfig(cfg *Config, replacements []DBReplace) ReplacementOptions {
	engine := strings.TrimSpace(cfg.DBReplaceEngine)
	if engine == "" {
		engine = defaultDBReplaceEngine(cfg, replacements)
	}

	skipColumns := append([]string{}, cfg.SkipColumns...)
	if !containsFold(skipColumns, "guid") {
		skipColumns = append(skipColumns, "guid")
	}

	return ReplacementOptions{
		Engine:             engine,
		ValidateSerialized: validateSerialized(cfg),
		Replacements:       replacements,
		SkipColumns:        skipColumns,
	}
}

func defaultDBReplaceEngine(cfg *Config, replacements []DBReplace) string {
	if len(replacements) == 0 {
		return DBReplaceEngineNone
	}
	if isWordPressLikeConfig(cfg) {
		return DBReplaceEngineGoSerialized
	}
	return DBReplaceEngineRaw
}

func validateSerialized(cfg *Config) bool {
	return cfg.ValidateSerialized == nil || *cfg.ValidateSerialized
}

func TransformSQLDump(input io.Reader, output io.Writer, options ReplacementOptions) error {
	switch options.Engine {
	case DBReplaceEngineNone:
		_, err := io.Copy(output, input)
		return err
	case DBReplaceEngineRaw, DBReplaceEngineGoSerialized, "":
	default:
		return fmt.Errorf("unsupported dbReplaceEngine %q", options.Engine)
	}

	options = prepareReplacementOptions(options)
	reader := bufio.NewReader(input)
	for {
		statement, readErr := readSQLStatement(reader)
		if len(statement) > 0 {
			transformed := statement
			var err error
			if options.Engine == DBReplaceEngineGoSerialized {
				transformed, err = transformInsertStatement(statement, options)
				if err != nil {
					return errors.Join(err, nonEOF(readErr))
				}
			} else {
				transformed = options.compiled.apply(statement)
			}
			if _, writeErr := io.WriteString(output, transformed); writeErr != nil {
				return errors.Join(writeErr, nonEOF(readErr))
			}
		}
		if readErr != nil {
			return nonEOF(readErr)
		}
	}
}

// Statement framing understands dump comments and SQL quote delimiters. Stored
// routines using client DELIMITER directives are outside the dump format.
func readSQLStatement(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	var quote byte
	escaped, lineComment, blockComment := false, false, false
	var previous byte
	for {
		b, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && (quote != 0 || blockComment) {
				return builder.String(), fmt.Errorf("truncated SQL quote or comment: %w", io.ErrUnexpectedEOF)
			}
			return builder.String(), err
		}
		builder.WriteByte(b)
		if lineComment {
			if b == '\n' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if previous == '*' && b == '/' {
				blockComment = false
				previous = 0
			} else {
				previous = b
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if b == '\\' && quote != '`' {
				escaped = true
				continue
			}
			if b == quote {
				next, _ := reader.Peek(1)
				if len(next) > 0 && next[0] == quote {
					nextByte, _ := reader.ReadByte()
					builder.WriteByte(nextByte)
				} else {
					quote = 0
				}
			}
			continue
		}
		switch b {
		case '\'', '"', '`':
			quote = b
		case '#':
			lineComment = true
		case '-':
			next, _ := reader.Peek(2)
			if len(next) == 2 && next[0] == '-' && next[1] <= ' ' {
				lineComment = true
			}
		case '/':
			next, _ := reader.Peek(1)
			if len(next) > 0 && next[0] == '*' {
				nextByte, _ := reader.ReadByte()
				builder.WriteByte(nextByte)
				blockComment = true
				previous = 0
			}
		case ';':
			return builder.String(), nil
		}
	}
}

func skipSQLTrivia(input string, pos int) int {
	for {
		pos = skipSQLSpaces(input, pos)
		rest := input[pos:]
		if strings.HasPrefix(rest, "#") || (strings.HasPrefix(rest, "--") && len(rest) > 2 && rest[2] <= ' ') {
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				return len(input)
			}
			pos += end + 1
		} else if strings.HasPrefix(rest, "/*") {
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				return len(input)
			}
			pos += end + 4
		} else {
			return pos
		}
	}
}

func transformInsertStatement(statement string, options ReplacementOptions) (string, error) {
	options = prepareReplacementOptions(options)
	offset := skipSQLTrivia(statement, 0)
	trimmed := statement[offset:]
	if !hasPrefixFold(trimmed, "INSERT INTO") {
		return statement, nil
	}

	relativeValues := findSQLKeyword(trimmed, "VALUES")
	valuesIndex := offset + relativeValues
	if relativeValues == -1 {
		return "", fmt.Errorf("parse INSERT values: missing VALUES")
	}

	head := statement[:valuesIndex]
	valuesPart := strings.TrimSpace(statement[valuesIndex+len("VALUES"):])
	suffix := ""
	if trimmedValues, ok := strings.CutSuffix(valuesPart, ";"); ok {
		valuesPart = strings.TrimSpace(trimmedValues)
		suffix = ";"
	}

	tableName, columns, err := parseInsertHeader(head[offset:])
	if err != nil {
		return "", fmt.Errorf("parse INSERT header: %w", err)
	}

	rows, err := parseSQLValues(valuesPart)
	if err == nil && len(rows) == 0 {
		err = fmt.Errorf("missing rows")
	}
	if err != nil {
		return "", fmt.Errorf("parse INSERT values: %w", err)
	}

	for rowIndex := range rows {
		if len(columns) > 0 && len(rows[rowIndex]) != len(columns) {
			return "", fmt.Errorf("parse INSERT values: row %d has %d values for %d columns", rowIndex+1, len(rows[rowIndex]), len(columns))
		}
		for columnIndex := range rows[rowIndex] {
			value := &rows[rowIndex][columnIndex]
			if !value.IsString {
				continue
			}
			if columnIndex < len(columns) {
				if _, skip := options.skipped[strings.ToLower(columns[columnIndex])]; skip {
					continue
				}
			}
			transformed, err := transformSQLString(value.String, options)
			if err != nil {
				return "", fmt.Errorf("transform table %s row %d column %d: %w", tableNameForError(tableName), rowIndex+1, columnIndex+1, err)
			}
			value.String = transformed
		}
	}

	return head + "VALUES " + formatSQLRows(rows) + suffix, nil
}

func parseInsertHeader(head string) (string, []string, error) {
	pos := skipSQLTrivia(head, len("INSERT INTO"))
	table, pos, err := parseSQLIdentifier(head, pos)
	if err != nil {
		return "", nil, err
	}
	pos = skipSQLTrivia(head, pos)
	if pos == len(head) {
		return table, nil, nil
	}
	if head[pos] != '(' {
		return "", nil, fmt.Errorf("expected column list")
	}
	pos++
	var columns []string
	for {
		pos = skipSQLTrivia(head, pos)
		column, next, err := parseSQLIdentifier(head, pos)
		if err != nil {
			return "", nil, err
		}
		columns = append(columns, column)
		pos = skipSQLTrivia(head, next)
		if pos == len(head) {
			return "", nil, fmt.Errorf("unterminated column list")
		}
		if head[pos] == ')' {
			pos++
			break
		}
		if head[pos] != ',' {
			return "", nil, fmt.Errorf("expected column separator")
		}
		pos++
	}
	if skipSQLTrivia(head, pos) != len(head) {
		return "", nil, fmt.Errorf("unexpected INSERT header suffix")
	}
	return table, columns, nil
}

func parseSQLIdentifier(input string, pos int) (string, int, error) {
	if pos >= len(input) {
		return "", pos, fmt.Errorf("missing SQL identifier")
	}
	if input[pos] == '`' {
		pos++
		var builder strings.Builder
		for pos < len(input) {
			b := input[pos]
			pos++
			if b == '`' {
				if pos < len(input) && input[pos] == '`' {
					builder.WriteByte('`')
					pos++
					continue
				}
				if builder.Len() == 0 {
					return "", pos, fmt.Errorf("empty SQL identifier")
				}
				return builder.String(), pos, nil
			}
			builder.WriteByte(b)
		}
		return "", pos, fmt.Errorf("unterminated quoted identifier")
	}
	start := pos
	for pos < len(input) && !unicode.IsSpace(rune(input[pos])) && !strings.ContainsRune("(),;", rune(input[pos])) {
		pos++
	}
	if pos == start {
		return "", pos, fmt.Errorf("missing SQL identifier")
	}
	return input[start:pos], pos, nil
}

func tableNameForError(tableName string) string {
	if tableName == "" {
		return "<unknown>"
	}
	return tableName
}

func parseSQLValues(input string) ([][]sqlValue, error) {
	var rows [][]sqlValue
	pos := 0
	for {
		pos = skipSQLSpaces(input, pos)
		if pos >= len(input) {
			break
		}
		if input[pos] != '(' {
			return nil, fmt.Errorf("expected row at byte %d", pos)
		}
		pos++

		var row []sqlValue
		for {
			pos = skipSQLSpaces(input, pos)
			if pos >= len(input) {
				return nil, fmt.Errorf("unterminated row")
			}

			var value sqlValue
			var err error
			if input[pos] == '\'' {
				value.String, pos, err = parseSQLString(input, pos)
				value.IsString = true
			} else {
				start := pos
				for pos < len(input) && input[pos] != ',' && input[pos] != ')' {
					pos++
				}
				value.Raw = strings.TrimSpace(input[start:pos])
				if value.Raw == "" {
					return nil, fmt.Errorf("empty SQL value at byte %d", start)
				}
			}
			if err != nil {
				return nil, err
			}
			row = append(row, value)

			pos = skipSQLSpaces(input, pos)
			if pos >= len(input) {
				return nil, fmt.Errorf("unterminated row")
			}
			switch input[pos] {
			case ',':
				pos++
			case ')':
				pos++
				rows = append(rows, row)
				goto nextRow
			default:
				return nil, fmt.Errorf("expected comma or row end at byte %d", pos)
			}
		}
	nextRow:
		pos = skipSQLSpaces(input, pos)
		if pos == len(input) {
			break
		}
		if input[pos] != ',' {
			return nil, fmt.Errorf("expected row separator at byte %d", pos)
		}
		pos = skipSQLSpaces(input, pos+1)
		if pos == len(input) {
			return nil, fmt.Errorf("trailing row separator")
		}
	}
	return rows, nil
}

func parseSQLString(input string, pos int) (string, int, error) {
	if pos >= len(input) || input[pos] != '\'' {
		return "", pos, fmt.Errorf("expected SQL string at byte %d", pos)
	}
	pos++
	var builder strings.Builder
	for pos < len(input) {
		b := input[pos]
		pos++
		if b == '\'' {
			if pos < len(input) && input[pos] == '\'' {
				builder.WriteByte('\'')
				pos++
				continue
			}
			return builder.String(), pos, nil
		}
		if b != '\\' {
			builder.WriteByte(b)
			continue
		}
		if pos >= len(input) {
			return "", pos, fmt.Errorf("unterminated SQL escape")
		}
		escaped := input[pos]
		pos++
		switch escaped {
		case '0':
			builder.WriteByte(0)
		case '\'':
			builder.WriteByte('\'')
		case '"':
			builder.WriteByte('"')
		case 'b':
			builder.WriteByte('\b')
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case 'Z':
			builder.WriteByte(26)
		case '\\':
			builder.WriteByte('\\')
		default:
			builder.WriteByte(escaped)
		}
	}
	return "", pos, fmt.Errorf("unterminated SQL string")
}

func formatSQLRows(rows [][]sqlValue) string {
	var builder strings.Builder
	for i, row := range rows {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteByte('(')
		for j, value := range row {
			if j > 0 {
				builder.WriteByte(',')
			}
			if value.IsString {
				writeSQLString(&builder, value.String)
			} else {
				builder.WriteString(value.Raw)
			}
		}
		builder.WriteByte(')')
	}
	return builder.String()
}

func quoteSQLString(value string) string {
	var builder strings.Builder
	writeSQLString(&builder, value)
	return builder.String()
}
func writeSQLString(builder *strings.Builder, value string) {
	builder.WriteByte('\'')
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case 0:
			builder.WriteString(`\0`)
		case '\'':
			builder.WriteString(`\'`)
		case '\\':
			builder.WriteString(`\\`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		case '\b':
			builder.WriteString(`\b`)
		case 26:
			builder.WriteString(`\Z`)
		default:
			builder.WriteByte(value[i])
		}
	}
	builder.WriteByte('\'')
}

func transformSQLString(value string, options ReplacementOptions) (string, error) {
	if isSerializedPHP(value) {
		transformed, err := transformSerializedPHPCompiled(value, prepareReplacementOptions(options).compiled)
		if err != nil {
			return value, nil
		}
		if options.ValidateSerialized && isSerializedPHP(transformed) {
			if _, err := parsePHPSerialized(transformed); err != nil {
				return "", fmt.Errorf("transformed serialized value is invalid: %w", err)
			}
		}
		return transformed, nil
	}
	return prepareReplacementOptions(options).compiled.apply(value), nil
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func findSQLKeyword(input, keyword string) int {
	inSingleQuote := false
	inBacktick := false
	escaped := false
	for i := 0; i <= len(input)-len(keyword); i++ {
		b := input[i]
		if inSingleQuote {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '\'' {
				inSingleQuote = false
			}
			continue
		}
		if inBacktick {
			if b == '`' {
				inBacktick = false
			}
			continue
		}
		if next := skipSQLTrivia(input, i); next > i {
			i = next - 1
			continue
		}
		switch b {
		case '\'':
			inSingleQuote = true
			continue
		case '`':
			inBacktick = true
			continue
		}
		if strings.EqualFold(input[i:i+len(keyword)], keyword) && isSQLBoundary(input, i-1) && isSQLBoundary(input, i+len(keyword)) {
			return i
		}
	}
	return -1
}

func isSQLBoundary(input string, pos int) bool {
	if pos < 0 || pos >= len(input) {
		return true
	}
	r := rune(input[pos])
	return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_')
}

func skipSQLSpaces(input string, pos int) int {
	for pos < len(input) && unicode.IsSpace(rune(input[pos])) {
		pos++
	}
	return pos
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}

type phpKind int

const (
	phpNull phpKind = iota
	phpBool
	phpInt
	phpFloat
	phpString
	phpArray
	phpObject
	phpReference
)

type phpValue struct {
	Kind          phpKind
	Bool          bool
	Int           int64
	Float         string
	String        string
	Pairs         []phpPair
	ClassName     string
	ReferenceType byte
	Reference     string
}

type phpPair struct {
	Key   phpValue
	Value phpValue
}

type phpParser struct {
	data  string
	pos   int
	depth int
}

func isSerializedPHP(value string) bool {
	value = strings.TrimSpace(value)
	if value == "N;" {
		return true
	}
	if len(value) < 4 || value[1] != ':' {
		return false
	}
	switch value[0] {
	case 'a', 'b', 'd', 'i', 'O', 's', 'R', 'r':
		return true
	default:
		return false
	}
}

func transformSerializedPHP(value string, replacements []DBReplace) (string, error) {
	return transformSerializedPHPCompiled(value, compileReplacements(replacements))
}
func transformSerializedPHPCompiled(value string, replacements compiledReplacements) (string, error) {
	parsed, err := parsePHPSerialized(value)
	if err != nil {
		return "", err
	}
	transformed, err := transformPHPValue(parsed, replacements, 0)
	if err != nil {
		return "", err
	}
	return serializePHPValue(transformed), nil
}

func parsePHPSerialized(value string) (phpValue, error) {
	parser := &phpParser{data: value}
	parsed, err := parser.parseValue()
	if err != nil {
		return phpValue{}, err
	}
	if parser.pos != len(value) {
		return phpValue{}, fmt.Errorf("trailing bytes at offset %d", parser.pos)
	}
	return parsed, nil
}

const maxSerializedDepth = 20

func (p *phpParser) parseValue() (phpValue, error) {
	if p.depth > maxSerializedDepth {
		return phpValue{}, fmt.Errorf("serialized recursion depth exceeded")
	}
	p.depth++
	defer func() { p.depth-- }()
	if p.pos >= len(p.data) {
		return phpValue{}, fmt.Errorf("unexpected end of serialized data")
	}
	if p.data[p.pos] != 'N' && (p.pos+1 >= len(p.data) || p.data[p.pos+1] != ':') {
		return phpValue{}, fmt.Errorf("missing serialized type separator")
	}
	switch p.data[p.pos] {
	case 'N':
		p.pos++
		if err := p.expect(';'); err != nil {
			return phpValue{}, err
		}
		return phpValue{Kind: phpNull}, nil
	case 'b':
		p.pos += 2
		value, err := p.readUntil(';')
		if err != nil {
			return phpValue{}, err
		}
		if value != "0" && value != "1" {
			return phpValue{}, fmt.Errorf("invalid boolean")
		}
		return phpValue{Kind: phpBool, Bool: value == "1"}, nil
	case 'i':
		p.pos += 2
		value, err := p.readUntil(';')
		if err != nil {
			return phpValue{}, err
		}
		integer, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return phpValue{}, err
		}
		return phpValue{Kind: phpInt, Int: integer}, nil
	case 'd':
		p.pos += 2
		value, err := p.readUntil(';')
		if err != nil {
			return phpValue{}, err
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return phpValue{}, err
		}
		return phpValue{Kind: phpFloat, Float: value}, nil
	case 's':
		return p.parseString()
	case 'a':
		return p.parseArray()
	case 'O':
		return p.parseObject()
	case 'R', 'r':
		return p.parseReference()
	default:
		return phpValue{}, fmt.Errorf("unsupported serialized type %q at offset %d", p.data[p.pos], p.pos)
	}
}

func (p *phpParser) parseString() (phpValue, error) {
	p.pos += 2
	lengthValue, err := p.readUntil(':')
	if err != nil {
		return phpValue{}, err
	}
	length, err := parsePHPSize(lengthValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if length > len(p.data)-p.pos {
		return phpValue{}, fmt.Errorf("string length %d exceeds remaining data", length)
	}
	value := p.data[p.pos : p.pos+length]
	p.pos += length
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if err := p.expect(';'); err != nil {
		return phpValue{}, err
	}
	return phpValue{Kind: phpString, String: value}, nil
}

func (p *phpParser) parseReference() (phpValue, error) {
	referenceType := p.data[p.pos]
	p.pos += 2
	reference, err := p.readUntil(';')
	if err != nil {
		return phpValue{}, err
	}
	if n, err := parsePHPSize(reference); err != nil || n == 0 {
		return phpValue{}, fmt.Errorf("invalid reference %q", reference)
	}
	return phpValue{Kind: phpReference, ReferenceType: referenceType, Reference: reference}, nil
}

func (p *phpParser) parseArray() (phpValue, error) {
	p.pos += 2
	countValue, err := p.readUntil(':')
	if err != nil {
		return phpValue{}, err
	}
	count, err := parsePHPSize(countValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('{'); err != nil {
		return phpValue{}, err
	}
	if count > (len(p.data)-p.pos)/4 {
		return phpValue{}, fmt.Errorf("pair count exceeds remaining data")
	}
	var pairs []phpPair
	for range count {
		key, err := p.parseValue()
		if err != nil {
			return phpValue{}, err
		}
		value, err := p.parseValue()
		if err != nil {
			return phpValue{}, err
		}
		pairs = append(pairs, phpPair{Key: key, Value: value})
	}
	if err := p.expect('}'); err != nil {
		return phpValue{}, err
	}
	return phpValue{Kind: phpArray, Pairs: pairs}, nil
}

func (p *phpParser) parseObject() (phpValue, error) {
	p.pos += 2
	classLengthValue, err := p.readUntil(':')
	if err != nil {
		return phpValue{}, err
	}
	classLength, err := parsePHPSize(classLengthValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if classLength > len(p.data)-p.pos {
		return phpValue{}, fmt.Errorf("object class length %d exceeds remaining data", classLength)
	}
	className := p.data[p.pos : p.pos+classLength]
	p.pos += classLength
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if err := p.expect(':'); err != nil {
		return phpValue{}, err
	}
	countValue, err := p.readUntil(':')
	if err != nil {
		return phpValue{}, err
	}
	count, err := parsePHPSize(countValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('{'); err != nil {
		return phpValue{}, err
	}
	if count > (len(p.data)-p.pos)/4 {
		return phpValue{}, fmt.Errorf("pair count exceeds remaining data")
	}
	var pairs []phpPair
	for range count {
		key, err := p.parseValue()
		if err != nil {
			return phpValue{}, err
		}
		value, err := p.parseValue()
		if err != nil {
			return phpValue{}, err
		}
		pairs = append(pairs, phpPair{Key: key, Value: value})
	}
	if err := p.expect('}'); err != nil {
		return phpValue{}, err
	}
	return phpValue{Kind: phpObject, ClassName: className, Pairs: pairs}, nil
}

func (p *phpParser) readUntil(delimiter byte) (string, error) {
	start := p.pos
	for p.pos < len(p.data) && p.data[p.pos] != delimiter {
		p.pos++
	}
	if p.pos >= len(p.data) {
		return "", fmt.Errorf("missing delimiter %q", delimiter)
	}
	value := p.data[start:p.pos]
	p.pos++
	return value, nil
}

func (p *phpParser) expect(want byte) error {
	if p.pos >= len(p.data) || p.data[p.pos] != want {
		return fmt.Errorf("expected %q at offset %d", want, p.pos)
	}
	p.pos++
	return nil
}

func transformPHPValue(value phpValue, replacements compiledReplacements, depth int) (phpValue, error) {
	if depth > maxSerializedDepth {
		return value, fmt.Errorf("serialized recursion depth exceeded")
	}
	switch value.Kind {
	case phpString:
		if isSerializedPHP(value.String) {
			nested, err := parsePHPSerialized(value.String)
			if err != nil {
				break
			}
			if err == nil {
				transformed, err := transformPHPValue(nested, replacements, depth+1)
				if err != nil {
					return value, err
				}
				value.String = serializePHPValue(transformed)
				break
			}
		}
		value.String = replacements.apply(value.String)
	case phpArray, phpObject:
		for i := range value.Pairs {
			key, err := transformPHPValue(value.Pairs[i].Key, replacements, depth+1)
			if err != nil {
				return value, err
			}
			child, err := transformPHPValue(value.Pairs[i].Value, replacements, depth+1)
			if err != nil {
				return value, err
			}
			value.Pairs[i].Key = key
			value.Pairs[i].Value = child
		}
	}
	return value, nil
}

func serializePHPValue(value phpValue) string {
	var builder strings.Builder
	writePHPValue(&builder, value)
	return builder.String()
}
func writePHPValue(builder *strings.Builder, value phpValue) {
	switch value.Kind {
	case phpNull:
		builder.WriteString("N;")
	case phpBool:
		if value.Bool {
			builder.WriteString("b:1;")
		} else {
			builder.WriteString("b:0;")
		}
	case phpInt:
		builder.WriteString("i:")
		builder.WriteString(strconv.FormatInt(value.Int, 10))
		builder.WriteByte(';')
	case phpFloat:
		builder.WriteString("d:")
		builder.WriteString(value.Float)
		builder.WriteByte(';')
	case phpString:
		builder.WriteString("s:")
		builder.WriteString(strconv.Itoa(len(value.String)))
		builder.WriteString(":\"")
		builder.WriteString(value.String)
		builder.WriteString("\";")
	case phpArray, phpObject:
		if value.Kind == phpObject {
			builder.WriteString("O:")
			builder.WriteString(strconv.Itoa(len(value.ClassName)))
			builder.WriteString(":\"")
			builder.WriteString(value.ClassName)
			builder.WriteString("\":")
		} else {
			builder.WriteString("a:")
		}
		builder.WriteString(strconv.Itoa(len(value.Pairs)))
		builder.WriteString(":{")
		for _, pair := range value.Pairs {
			writePHPValue(builder, pair.Key)
			writePHPValue(builder, pair.Value)
		}
		builder.WriteByte('}')
	case phpReference:
		builder.WriteByte(value.ReferenceType)
		builder.WriteByte(':')
		builder.WriteString(value.Reference)
		builder.WriteByte(';')
	}
}

func nonEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
func parsePHPSize(value string) (int, error) {
	if value == "" || strings.Trim(value, "0123456789") != "" {
		return 0, fmt.Errorf("invalid serialized size %q", value)
	}
	return strconv.Atoi(value)
}
func prepareReplacementOptions(options ReplacementOptions) ReplacementOptions {
	if options.compiled == nil {
		options.compiled = compileReplacements(options.Replacements)
	}
	if options.skipped == nil {
		options.skipped = make(map[string]struct{}, len(options.SkipColumns))
		for _, column := range options.SkipColumns {
			options.skipped[strings.ToLower(column)] = struct{}{}
		}
	}
	return options
}
