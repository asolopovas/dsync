package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func SyncFiles(ctx context.Context, cfg *Config, reverse bool) error {
	if reverse {
		fmt.Println("Syncing Files from local to remote using rsync")
	} else {
		fmt.Println("Syncing Files from remote to local using rsync")
	}
	fmt.Println(strings.Repeat("-", 50))

	maxLen := 0
	for _, item := range cfg.Sync {
		if len(item.Remote) > maxLen {
			maxLen = len(item.Remote)
		}
	}

	for _, item := range cfg.Sync {
		remotePath := ensureTrailingSlash(item.Remote)
		localPath := ensureTrailingSlash(item.Local)

		if reverse {
			fmt.Printf("%-*s -> %s\n", maxLen, localPath, remotePath)
		} else {
			fmt.Printf("%-*s -> %s\n", maxLen, remotePath, localPath)
		}

		if len(item.Exclude) > 0 {
			fmt.Println("  Excluding:")
			for _, v := range item.Exclude {
				fmt.Printf("    - %s\n", v)
			}
		}

		if err := runRsync(ctx, cfg, item, remotePath, localPath, reverse); err != nil {
			fmt.Printf("Error syncing %s: %v\n", remotePath, err)
			// Continue syncing other paths? Original code did.
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func ensureTrailingSlash(s string) string {
	if strings.HasSuffix(s, "/") {
		return s
	}
	return s + "/"
}
