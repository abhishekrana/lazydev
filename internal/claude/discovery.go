// Package claude integrates Claude Code (the Anthropic CLI) with
// lazydev. It owns binary discovery, structured prompt composition,
// session persistence in `.lazydev/sessions.json`, and the foreground
// dispatch paths (one-shot `claude -p` and interactive tmux split).
//
// Lazydev does NOT manage Claude Code's own session store under
// `~/.claude/projects/<encoded>/` — that remains authoritative for
// resume. We track our dispatches separately so the user can see them
// listed in the UI.
package claude

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// Env describes what is available on the host: which binaries we
// resolved and the path to the git working tree (or $PWD if not in a
// repo). It is built once at startup and passed through.
type Env struct {
	// ClaudeBin is the resolved absolute path of the claude binary, or
	// empty when not on PATH.
	ClaudeBin string
	// TmuxBin is the resolved absolute path of tmux, or empty.
	TmuxBin string
	// RepoRoot is the directory we treat as the project root (git
	// toplevel preferred, $PWD fallback).
	RepoRoot string
	// InsideTmux is true when the lazydev process itself is running
	// under tmux (TMUX env var set).
	InsideTmux bool
}

// Discover resolves binaries and the repo root. `claudeBin` is the
// configured binary name (default "claude"); pass "" to use the default.
func Discover(claudeBin string) Env {
	if claudeBin == "" {
		claudeBin = "claude"
	}
	env := Env{}
	if p, err := exec.LookPath(claudeBin); err == nil {
		env.ClaudeBin = p
	}
	if p, err := exec.LookPath("tmux"); err == nil {
		env.TmuxBin = p
	}
	env.RepoRoot = gitRoot()
	env.InsideTmux = os.Getenv("TMUX") != ""
	return env
}

// ClaudeAvailable reports whether the claude binary was found.
func (e Env) ClaudeAvailable() bool { return e.ClaudeBin != "" }

// TmuxAvailable reports whether tmux is on PATH.
func (e Env) TmuxAvailable() bool { return e.TmuxBin != "" }

// gitRoot returns the git working tree root, or "" on failure.
func gitRoot() string {
	c := exec.Command("git", "rev-parse", "--show-toplevel") //nolint:gosec,noctx // fixed args
	var out bytes.Buffer
	c.Stdout = &out
	if err := c.Run(); err != nil {
		if cwd, _ := os.Getwd(); cwd != "" {
			return cwd
		}
		return ""
	}
	return strings.TrimSpace(out.String())
}
