package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pterm/pterm"
)

func SyncFiles(ctx context.Context, cfg *Config, reverse bool) error {
	direction := "remote to local"
	if reverse {
		direction = "local to remote"
	}
	pterm.DefaultSection.Printf("Syncing Files (%s)\n", direction)

	for _, item := range cfg.Sync {
		if err := ctx.Err(); err != nil {
			return err
		}
		remotePath := ensureTrailingSlash(item.Remote)
		localPath := ensureTrailingSlash(item.Local)

		var msg string
		if reverse {
			msg = fmt.Sprintf("%s -> %s", localPath, remotePath)
		} else {
			msg = fmt.Sprintf("%s -> %s", remotePath, localPath)
		}

		pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
			{Level: 0, Text: msg, TextStyle: pterm.NewStyle(pterm.FgCyan)},
		}).Render()

		var details []pterm.BulletListItem
		for _, v := range item.Exclude {
			details = append(details, pterm.BulletListItem{Level: 1, Text: "Exclude: " + v, TextStyle: pterm.NewStyle(pterm.FgGray)})
		}
		if item.Replace {
			details = append(details, pterm.BulletListItem{Level: 1, Text: "Replace synced text file URLs", TextStyle: pterm.NewStyle(pterm.FgGray)})
		}
		if len(details) > 0 {
			pterm.DefaultBulletList.WithItems(details).Render()
		}

		spinner := startSpinner("Running rsync...")
		if err := runRsync(ctx, cfg, item, remotePath, localPath, reverse); err != nil {
			spinner.Fail(fmt.Sprintf("Rsync failed: %v", err))
			return err
		} else {
			spinner.Success("Rsync completed")
		}

		if item.Replace && !reverse && len(cfg.DBReplace) > 0 {
			spinner := startSpinner("Applying replacements to synced text files...")
			changed, err := applyFileReplacementsContext(ctx, localPath, cfg.DBReplace, item.Exclude)
			if err != nil {
				spinner.Fail(fmt.Sprintf("File replacements failed: %v", err))
				return err
			}
			spinner.Success(fmt.Sprintf("Updated %d synced text files", changed))
		}
		fmt.Println()
	}
	return nil
}

func rsyncArgs(cfg *Config, item SyncPath, remotePath, localPath string, reverse bool) []string {
	args := []string{
		"-azr", "--protect-args",
		"-e", "ssh -p " + shellQuote(cfg.Port),
		"--info=progress2",
	}

	for _, v := range item.Exclude {
		args = append(args, "--exclude="+v)
	}

	args = append(args, "--")
	if strings.HasPrefix(remotePath, ":") {
		remotePath = "./" + remotePath
	}
	if !filepath.IsAbs(localPath) {
		localPath = "./" + localPath
	}
	if reverse {
		args = append(args, localPath, cfg.SSHHost+":"+remotePath)
	} else {
		args = append(args, cfg.SSHHost+":"+remotePath, localPath)
	}

	return args
}

func runRsync(ctx context.Context, cfg *Config, item SyncPath, remotePath, localPath string, reverse bool) error {
	cmd := exec.CommandContext(ctx, "rsync", rsyncArgs(cfg, item, remotePath, localPath, reverse)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync failed: %s: %w", output, errors.Join(err, ctx.Err()))
	}

	return nil
}

func ensureTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}

func applyFileReplacements(root string, replacements []DBReplace) (int, error) {
	return applyFileReplacementsContext(context.Background(), root, replacements, nil)
}

func applyFileReplacementsContext(ctx context.Context, root string, replacements []DBReplace, excludes []string) (int, error) {
	if len(replacements) == 0 {
		return 0, nil
	}

	compiled := compileReplacements(replacements)
	filters := compileExclusions(excludes)
	root = filepath.Clean(root)
	changed := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && isExcluded(filters, filepath.ToSlash(rel), entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !isTextReplacementCandidate(path) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		original := string(data)
		updated := compiled.apply(original)
		if updated == original {
			return nil
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		if err := replaceFileAtomically(path, []byte(updated), info); err != nil {
			return err
		}

		changed++
		return nil
	})
	if err != nil {
		return changed, fmt.Errorf("apply replacements under %s: %w", root, err)
	}

	return changed, nil
}

func isTextReplacementCandidate(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".css", ".js", ".json", ".html", ".htm", ".svg", ".xml", ".txt", ".map", ".php", ".twig":
		return true
	default:
		return false
	}
}

func replaceFileAtomically(path string, data []byte, info os.FileInfo) (err error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".dsync-replace-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err == nil {
		err = file.Chmod(info.Mode())
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return err
	}
	if err := os.Chtimes(file.Name(), info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

type exclusion struct {
	pattern   *regexp.Regexp
	directory bool
	include   bool
}

// Rsync patterns without a slash match any basename; slash-containing patterns
// match path suffixes, and a leading slash anchors at the transfer root.
func compileExclusions(patterns []string) []exclusion {
	var result []exclusion
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if pattern == "!" {
			result = nil
			continue
		}
		include := strings.HasPrefix(pattern, "+ ")
		if include || strings.HasPrefix(pattern, "- ") {
			pattern = pattern[2:]
		}
		directory := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")
		anchored := strings.HasPrefix(pattern, "/")
		pattern = strings.TrimPrefix(pattern, "/")
		originalPattern := pattern
		wildcard := strings.ContainsAny(pattern, "*?[")
		prefix := "(?:^|/)"
		if anchored {
			prefix = "^"
		}
		var regex strings.Builder
		regex.WriteString("(?s)" + prefix)
		if strings.HasPrefix(pattern, "**/") {
			regex.WriteString("(?:.*/)?")
			pattern = pattern[3:]
		}
		for i := 0; i < len(pattern); i++ {
			switch pattern[i] {
			case '*':
				if i+1 < len(pattern) && pattern[i+1] == '*' {
					i++
					if i+1 < len(pattern) && pattern[i+1] == '*' {
						i++
					}
					regex.WriteString(".*")
				} else {
					regex.WriteString("[^/]*")
				}
			case '?':
				regex.WriteString("[^/]")
			case '[':
				end := -1
				for j := i + 1; j < len(pattern); j++ {
					if pattern[j] == '[' && j+1 < len(pattern) && pattern[j+1] == ':' {
						if n := strings.Index(pattern[j+2:], ":]"); n >= 0 {
							j += n + 3
							continue
						}
					}
					first := i + 1
					if first < len(pattern) && (pattern[first] == '!' || pattern[first] == '^') {
						first++
					}
					if pattern[j] == ']' && j > first {
						end = j - i - 1
						break
					}
				}
				if end < 0 {
					regex.WriteString(`\[`)
					continue
				}
				class := pattern[i+1 : i+1+end]
				if strings.HasPrefix(class, "!") {
					class = "^" + class[1:]
				}
				regex.WriteByte('[')
				regex.WriteString(class)
				regex.WriteByte(']')
				i += end + 1
			case '\\':
				if !wildcard {
					regex.WriteString(`\\`)
					continue
				}
				if i+1 < len(pattern) {
					i++
					regex.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
				} else {
					regex.WriteString(`\\`)
				}
			default:
				regex.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			}
		}
		regex.WriteByte('$')
		re, err := regexp.Compile(regex.String())
		if err == nil {
			result = append(result, exclusion{pattern: re, directory: directory, include: include})
		}
		// dir/*** also excludes the directory itself.
		if strings.HasSuffix(originalPattern, "/***") {
			directoryPattern := strings.TrimSuffix(regexPrefix(anchored)+originalPattern, "/***") + "/"
			if include {
				directoryPattern = "+ " + directoryPattern
			}
			result = append(result, compileExclusions([]string{directoryPattern})...)
		}
	}
	return result
}
func regexPrefix(anchored bool) string {
	if anchored {
		return "/"
	}
	return ""
}
func isExcluded(filters []exclusion, path string, directory bool) bool {
	for _, filter := range filters {
		if (!filter.directory || directory) && filter.pattern.MatchString(path) {
			return !filter.include
		}
	}
	return false
}
