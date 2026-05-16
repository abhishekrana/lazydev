package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed skill.md
var skillMD string

func cmdInstallSkill(args []string) int {
	fs := flag.NewFlagSet("install-skill", flag.ContinueOnError)
	force := fs.Bool("force", false, "overwrite an existing SKILL.md")
	printOnly := fs.Bool("print", false, "print the embedded SKILL.md to stdout and exit")
	target := fs.String("path", "", "override the install path (default: ~/.claude/skills/lazydev/SKILL.md)")
	usage(fs,
		"Usage: lazydev install-skill [--force] [--print] [--path PATH]",
		"",
		"Write the lazydev Claude Code skill to ~/.claude/skills/lazydev/SKILL.md",
		"so Claude Code learns when/how to query the local cache. The skill text",
		"is embedded in this binary; no repo files are required on the target",
		"machine.",
	)
	if err := fs.Parse(reorderFlags(args, map[string]bool{"force": true, "print": true})); err != nil {
		return 2
	}

	if *printOnly {
		if _, err := os.Stdout.WriteString(skillMD); err != nil {
			fail(err)
			return 1
		}
		return 0
	}

	path := *target
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fail(fmt.Errorf("resolving home: %w", err))
			return 1
		}
		path = filepath.Join(home, ".claude", "skills", "lazydev", "SKILL.md")
	}

	if _, err := os.Stat(path); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "lazydev: %s already exists — pass --force to overwrite\n", path)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		fail(err)
		return 1
	}
	if err := os.WriteFile(path, []byte(skillMD), 0o644); err != nil { //nolint:gosec // user-owned skill file
		fail(err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "Installed lazydev skill at %s\n", path)
	return 0
}
