package claude

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoClaude is returned by dispatch helpers when the claude binary
// could not be resolved at startup. The UI surfaces this verbatim.
var ErrNoClaude = errors.New("claude not in PATH (install: https://docs.anthropic.com/claude-code)")

// DispatchRequest carries everything needed to start a session and
// persist its record. Callers fill it in from the active tab.
type DispatchRequest struct {
	Env     Env
	Store   *Store
	Mode    Mode
	Kind    string // "issue" | "mr"
	Ref     string // "#421" | "!1024"
	Title   string
	Prompt  string
	Session string // tmux session base name (e.g. "lazydev-claude")
}

// Result captures the outcome of a dispatch — used by tabs to drive
// notifications and (for one-shot) to surface stdout location.
type Result struct {
	Session    Session
	AttachHint string // for interactive: "tmux attach -t <target>"
	LogPath    string // for one-shot
	Note       string // human-readable summary
}

// DispatchOneShot runs `claude -p <prompt>` foregrounded. Stdout +
// stderr are captured to `.lazydev/claude-runs/<id>.log`. Blocks until
// the child exits.
func DispatchOneShot(req DispatchRequest) (Result, error) {
	if !req.Env.ClaudeAvailable() {
		return Result{}, ErrNoClaude
	}
	id := NewID()
	now := time.Now()
	logDir := filepath.Join(req.Env.RepoRoot, ".lazydev", "claude-runs")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return Result{}, err
	}
	logPath := filepath.Join(logDir, id+".log")
	logFile, err := os.Create(logPath) //nolint:gosec // path is repo-relative + lazydev-owned
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = logFile.Close() }()

	sess := Session{
		ID:         id,
		Kind:       req.Kind,
		Ref:        req.Ref,
		Title:      req.Title,
		Mode:       ModeOneShot,
		LogPath:    logPath,
		Status:     StatusRunning,
		CreatedAt:  now,
		LastSeenAt: now,
	}
	if err := req.Store.Add(sess); err != nil {
		return Result{}, err
	}

	c := exec.Command(req.Env.ClaudeBin, "-p", req.Prompt) //nolint:gosec,noctx // user-driven dispatch
	c.Dir = req.Env.RepoRoot
	c.Stdout = logFile
	c.Stderr = logFile
	runErr := c.Run()

	status := StatusDone
	note := fmt.Sprintf("one-shot %s done → %s", req.Ref, logPath)
	if runErr != nil {
		status = StatusFailed
		note = fmt.Sprintf("one-shot %s failed: %v", req.Ref, runErr)
	}
	_ = req.Store.Update(id, func(s *Session) {
		s.Status = status
		s.LastSeenAt = time.Now()
		if runErr != nil {
			s.ExitNote = runErr.Error()
		}
	})
	sess.Status = status
	return Result{Session: sess, LogPath: logPath, Note: note}, nil
}

// DispatchInteractive spawns Claude Code in a tmux split (or new tmux
// session if lazydev is running outside tmux). The structured prompt is
// written to a tempfile and passed as the initial argument; Claude
// Code reads the file path as a task input on startup.
//
// Falls back to ErrNoTmux behavior by escalating to one-shot when tmux
// is unavailable — the caller decides whether to invoke the fallback.
func DispatchInteractive(req DispatchRequest) (Result, error) {
	if !req.Env.ClaudeAvailable() {
		return Result{}, ErrNoClaude
	}
	if !req.Env.TmuxAvailable() {
		return Result{}, errors.New("tmux not in PATH — install tmux or use 'P' for one-shot")
	}

	id := NewID()
	now := time.Now()
	promptDir := filepath.Join(req.Env.RepoRoot, ".lazydev", "claude-prompts")
	if err := os.MkdirAll(promptDir, 0o750); err != nil {
		return Result{}, err
	}
	promptPath := filepath.Join(promptDir, id+".md")
	if err := os.WriteFile(promptPath, []byte(req.Prompt), 0o600); err != nil { //nolint:gosec // repo-owned
		return Result{}, err
	}

	base := req.Session
	if base == "" {
		base = "lazydev-claude"
	}
	// Build the command Claude Code will run inside the tmux pane. We
	// `cat` the prompt then exec claude — this gives the user a visible
	// view of the task and starts an interactive session in the repo.
	cdQuoted := shellQuote(req.Env.RepoRoot)
	promptQuoted := shellQuote(promptPath)
	claudeQuoted := shellQuote(req.Env.ClaudeBin)
	inner := fmt.Sprintf(
		"cd %s && cat %s && echo && echo '--- starting claude ---' && exec %s %s",
		cdQuoted, promptQuoted, claudeQuoted, promptQuoted,
	)

	var target string
	if req.Env.InsideTmux {
		// Split the current window vertically and run there.
		windowName := "claude-" + id
		args := []string{
			"new-window", "-d",
			"-n", windowName,
			"-c", req.Env.RepoRoot,
			inner,
		}
		c := exec.Command(req.Env.TmuxBin, args...) //nolint:gosec,noctx // tmux call
		if out, err := c.CombinedOutput(); err != nil {
			return Result{}, fmt.Errorf("tmux new-window: %v: %s", err, strings.TrimSpace(string(out)))
		}
		target = ":" + windowName
	} else {
		// Detached session the user can attach to from outside lazydev.
		sessName := fmt.Sprintf("%s-%s", base, id)
		args := []string{
			"new-session", "-d",
			"-s", sessName,
			"-c", req.Env.RepoRoot,
			inner,
		}
		c := exec.Command(req.Env.TmuxBin, args...) //nolint:gosec,noctx // tmux call
		if out, err := c.CombinedOutput(); err != nil {
			return Result{}, fmt.Errorf("tmux new-session: %v: %s", err, strings.TrimSpace(string(out)))
		}
		target = sessName
	}

	sess := Session{
		ID:         id,
		Kind:       req.Kind,
		Ref:        req.Ref,
		Title:      req.Title,
		Mode:       ModeInteractive,
		TmuxTarget: target,
		PromptPath: promptPath,
		Status:     StatusRunning,
		CreatedAt:  now,
		LastSeenAt: now,
	}
	if err := req.Store.Add(sess); err != nil {
		return Result{}, err
	}

	hint := fmt.Sprintf("tmux attach -t %s", target)
	if req.Env.InsideTmux {
		hint = fmt.Sprintf("(prefix) + : select-window -t %s", target)
	}
	note := fmt.Sprintf("claude session %s started → %s", id[:6], hint)
	return Result{Session: sess, AttachHint: hint, Note: note}, nil
}

// AttachCommand returns the exec.Cmd that re-attaches to an existing
// session. Returns nil if the session has no tmux target.
func AttachCommand(env Env, sess Session) *exec.Cmd {
	if sess.TmuxTarget == "" || !env.TmuxAvailable() {
		return nil
	}
	// `tmux attach -t <target>` works for both session names and
	// `:windowname` inside an existing client.
	target := sess.TmuxTarget
	if strings.HasPrefix(target, ":") {
		return exec.Command(env.TmuxBin, "select-window", "-t", target) //nolint:gosec,noctx // tmux call
	}
	return exec.Command(env.TmuxBin, "attach", "-t", target) //nolint:gosec,noctx // tmux call
}

// shellQuote wraps s in single quotes, escaping any embedded single
// quote per POSIX `'\”` convention. Adequate for paths and binaries
// we constructed; not a general shell escaper.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
