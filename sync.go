package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
)

func SyncFiles(ctx context.Context, cfg *Config, reverse bool) error {
	direction := "remote to local"
	if reverse {
		direction = "local to remote"
	}
	pterm.DefaultSection.Printf("Syncing Files (%s)\n", direction)

	maxLen := 0
	for _, item := range cfg.Sync {
		if len(item.Remote) > maxLen {
			maxLen = len(item.Remote)
		}
	}

	for _, item := range cfg.Sync {
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

		spinner, _ := pterm.DefaultSpinner.Start("Running rsync...")
		if err := runRsync(ctx, cfg, item, remotePath, localPath, reverse); err != nil {
			spinner.Fail(fmt.Sprintf("Rsync failed: %v", err))
		} else {
			spinner.Success("Rsync completed")
		}

		if item.Replace && !reverse && len(cfg.DBReplace) > 0 {
			spinner, _ := pterm.DefaultSpinner.Start("Applying replacements to synced text files...")
			changed, err := applyFileReplacements(localPath, cfg.DBReplace)
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

func runRsync(ctx context.Context, cfg *Config, item SyncPath, remotePath, localPath string, reverse bool) error {
	args := []string{
		"-azr",
		"-e", "ssh -p " + cfg.Port,
		"--info=progress2",
	}

	for _, v := range item.Exclude {
		args = append(args, "--exclude="+v)
	}

	if reverse {
		args = append(args, localPath, cfg.SSHHost+":"+remotePath)
	} else {
		args = append(args, cfg.SSHHost+":"+remotePath, localPath)
	}

	cmd := exec.CommandContext(ctx, "rsync", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
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
	if len(replacements) == 0 {
		return 0, nil
	}

	changed := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
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
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		original := string(data)
		updated := applyStringReplacements(original, replacements)
		if updated == original {
			return nil
		}

		if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
			return err
		}
		if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
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
