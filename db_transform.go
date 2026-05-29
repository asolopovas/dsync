package main

import (
	"bufio"
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

	reader := bufio.NewReader(input)
	for {
		statement, err := readSQLStatement(reader)
		if len(statement) > 0 {
			transformed := statement
			if options.Engine == DBReplaceEngineGoSerialized {
				transformed, err = transformInsertStatement(statement, options)
				if err != nil {
					return err
				}
			} else {
				transformed = applyStringReplacements(statement, options.Replacements)
			}
			if _, writeErr := io.WriteString(output, transformed); writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readSQLStatement(reader *bufio.Reader) (string, error) {
	var builder strings.Builder
	inSingleQuote := false
	escaped := false

	for {
		b, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF && builder.Len() > 0 {
				return builder.String(), io.EOF
			}
			return builder.String(), err
		}

		builder.WriteByte(b)
		if inSingleQuote {
			if escaped {
				escaped = false
				continue
			}
			switch b {
			case '\\':
				escaped = true
			case '\'':
				inSingleQuote = false
			}
			continue
		}

		switch b {
		case '\'':
			inSingleQuote = true
		case ';':
			return builder.String(), nil
		}
	}
}

func transformInsertStatement(statement string, options ReplacementOptions) (string, error) {
	trimmed := strings.TrimLeftFunc(statement, unicode.IsSpace)
	if !hasPrefixFold(trimmed, "INSERT INTO") {
		return statement, nil
	}

	valuesIndex := findSQLKeyword(statement, "VALUES")
	if valuesIndex == -1 {
		return statement, nil
	}

	head := statement[:valuesIndex]
	valuesPart := strings.TrimSpace(statement[valuesIndex+len("VALUES"):])
	suffix := ""
	if trimmedValues, ok := strings.CutSuffix(valuesPart, ";"); ok {
		valuesPart = strings.TrimSpace(trimmedValues)
		suffix = ";"
	}

	tableName, columns, err := parseInsertHeader(head)
	if err != nil {
		return "", fmt.Errorf("parse INSERT header: %w", err)
	}

	rows, err := parseSQLValues(valuesPart)
	if err != nil {
		return "", fmt.Errorf("parse INSERT values: %w", err)
	}

	skipColumns := make(map[string]struct{}, len(options.SkipColumns))
	for _, column := range options.SkipColumns {
		skipColumns[strings.ToLower(column)] = struct{}{}
	}

	for rowIndex := range rows {
		for columnIndex := range rows[rowIndex] {
			value := &rows[rowIndex][columnIndex]
			if !value.IsString {
				continue
			}
			if columnIndex < len(columns) {
				if _, skip := skipColumns[strings.ToLower(columns[columnIndex])]; skip {
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
	afterIntoIndex := indexFold(head, "INSERT INTO")
	if afterIntoIndex == -1 {
		return "", nil, nil
	}

	pos := afterIntoIndex + len("INSERT INTO")
	pos = skipSQLSpaces(head, pos)
	if pos >= len(head) {
		return "", nil, nil
	}

	tableName, nextPos, err := parseSQLIdentifier(head, pos)
	if err != nil {
		return "", nil, err
	}
	pos = skipSQLSpaces(head, nextPos)
	if pos >= len(head) || head[pos] != '(' {
		return tableName, nil, nil
	}

	end := findMatchingParen(head, pos)
	if end == -1 {
		return tableName, nil, fmt.Errorf("unterminated column list")
	}

	parts := strings.Split(head[pos+1:end], ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "`")
		if part != "" {
			columns = append(columns, part)
		}
	}
	return tableName, columns, nil
}

func parseSQLIdentifier(input string, pos int) (string, int, error) {
	if input[pos] == '`' {
		pos++
		start := pos
		for pos < len(input) && input[pos] != '`' {
			pos++
		}
		if pos >= len(input) {
			return "", pos, fmt.Errorf("unterminated quoted identifier")
		}
		return input[start:pos], pos + 1, nil
	}

	start := pos
	for pos < len(input) && !unicode.IsSpace(rune(input[pos])) && input[pos] != '(' {
		pos++
	}
	return strings.Trim(input[start:pos], "`"), pos, nil
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
		pos = skipSQLSpacesAndCommas(input, pos)
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
	formattedRows := make([]string, 0, len(rows))
	for _, row := range rows {
		values := make([]string, 0, len(row))
		for _, value := range row {
			if value.IsString {
				values = append(values, quoteSQLString(value.String))
			} else {
				values = append(values, value.Raw)
			}
		}
		formattedRows = append(formattedRows, "("+strings.Join(values, ",")+")")
	}
	return strings.Join(formattedRows, ",")
}

func quoteSQLString(value string) string {
	var builder strings.Builder
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
	return builder.String()
}

func transformSQLString(value string, options ReplacementOptions) (string, error) {
	if isSerializedPHP(value) {
		transformed, err := transformSerializedPHP(value, options.Replacements)
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
	return applyStringReplacements(value, options.Replacements), nil
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func indexFold(value, needle string) int {
	return strings.Index(strings.ToLower(value), strings.ToLower(needle))
}

func findSQLKeyword(input, keyword string) int {
	upperKeyword := strings.ToUpper(keyword)
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
		switch b {
		case '\'':
			inSingleQuote = true
			continue
		case '`':
			inBacktick = true
			continue
		}
		if strings.ToUpper(input[i:i+len(keyword)]) == upperKeyword && isSQLBoundary(input, i-1) && isSQLBoundary(input, i+len(keyword)) {
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

func skipSQLSpacesAndCommas(input string, pos int) int {
	for pos < len(input) && (unicode.IsSpace(rune(input[pos])) || input[pos] == ',') {
		pos++
	}
	return pos
}

func findMatchingParen(input string, start int) int {
	depth := 0
	for i := start; i < len(input); i++ {
		switch input[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
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
	data string
	pos  int
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

func (p *phpParser) parseValue() (phpValue, error) {
	if p.pos >= len(p.data) {
		return phpValue{}, fmt.Errorf("unexpected end of serialized data")
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
	length, err := strconv.Atoi(lengthValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if p.pos+length > len(p.data) {
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
	if _, err := strconv.Atoi(reference); err != nil {
		return phpValue{}, err
	}
	return phpValue{Kind: phpReference, ReferenceType: referenceType, Reference: reference}, nil
}

func (p *phpParser) parseArray() (phpValue, error) {
	p.pos += 2
	countValue, err := p.readUntil(':')
	if err != nil {
		return phpValue{}, err
	}
	count, err := strconv.Atoi(countValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('{'); err != nil {
		return phpValue{}, err
	}
	pairs := make([]phpPair, 0, count)
	for i := 0; i < count; i++ {
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
	classLength, err := strconv.Atoi(classLengthValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('"'); err != nil {
		return phpValue{}, err
	}
	if p.pos+classLength > len(p.data) {
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
	count, err := strconv.Atoi(countValue)
	if err != nil {
		return phpValue{}, err
	}
	if err := p.expect('{'); err != nil {
		return phpValue{}, err
	}
	pairs := make([]phpPair, 0, count)
	for i := 0; i < count; i++ {
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

func transformPHPValue(value phpValue, replacements []DBReplace, depth int) (phpValue, error) {
	if depth > 20 {
		return value, fmt.Errorf("serialized recursion depth exceeded")
	}
	switch value.Kind {
	case phpString:
		if isSerializedPHP(value.String) {
			nested, err := parsePHPSerialized(value.String)
			if err == nil {
				transformed, err := transformPHPValue(nested, replacements, depth+1)
				if err != nil {
					return value, err
				}
				value.String = serializePHPValue(transformed)
				break
			}
		}
		value.String = applyStringReplacements(value.String, replacements)
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
	switch value.Kind {
	case phpNull:
		return "N;"
	case phpBool:
		if value.Bool {
			return "b:1;"
		}
		return "b:0;"
	case phpInt:
		return fmt.Sprintf("i:%d;", value.Int)
	case phpFloat:
		return "d:" + value.Float + ";"
	case phpString:
		return fmt.Sprintf("s:%d:\"%s\";", len([]byte(value.String)), value.String)
	case phpArray:
		var builder strings.Builder
		fmt.Fprintf(&builder, "a:%d:{", len(value.Pairs))
		for _, pair := range value.Pairs {
			builder.WriteString(serializePHPValue(pair.Key))
			builder.WriteString(serializePHPValue(pair.Value))
		}
		builder.WriteByte('}')
		return builder.String()
	case phpObject:
		var builder strings.Builder
		fmt.Fprintf(&builder, "O:%d:\"%s\":%d:{", len([]byte(value.ClassName)), value.ClassName, len(value.Pairs))
		for _, pair := range value.Pairs {
			builder.WriteString(serializePHPValue(pair.Key))
			builder.WriteString(serializePHPValue(pair.Value))
		}
		builder.WriteByte('}')
		return builder.String()
	case phpReference:
		return fmt.Sprintf("%c:%s;", value.ReferenceType, value.Reference)
	default:
		return "N;"
	}
}
