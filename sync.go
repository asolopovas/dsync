package main

import (
	"context"
	"fmt"
	"os/exec"
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

		if len(item.Exclude) > 0 {
			var excludes []pterm.BulletListItem
			for _, v := range item.Exclude {
				excludes = append(excludes, pterm.BulletListItem{Level: 1, Text: "Exclude: " + v, TextStyle: pterm.NewStyle(pterm.FgGray)})
			}
			pterm.DefaultBulletList.WithItems(excludes).Render()
		}

		spinner, _ := pterm.DefaultSpinner.Start("Running rsync...")
		if err := runRsync(ctx, cfg, item, remotePath, localPath, reverse); err != nil {
			spinner.Fail(fmt.Sprintf("Rsync failed: %v", err))
		} else {
			spinner.Success("Rsync completed")
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
