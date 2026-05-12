package tabs

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/abhishek-rana/lazydev/internal/claude"
	"github.com/abhishek-rana/lazydev/internal/export"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// kindIssue and kindMR are the two ExportItem kinds the tab package
// passes through to claude.DispatchRequest.
const (
	kindIssue = "issue"
	kindMR    = "mr"
)

// dispatchClaude composes a structured prompt from the given export
// items and either spawns Claude Code in a tmux split (interactive) or
// runs `claude -p` foreground (one-shot). Used by Issues and MRs tabs
// behind their `C` and `P` keys.
func dispatchClaude(opts *Options, mode claude.Mode, items []export.ExportItem) tea.Cmd {
	if len(items) == 0 {
		return func() tea.Msg {
			return messages.ClaudeDispatchMsg{Err: errors.New(nothingToExport)}
		}
	}
	if opts == nil || !opts.ClaudeEnv.ClaudeAvailable() {
		return func() tea.Msg {
			return messages.ClaudeDispatchMsg{Err: claude.ErrNoClaude}
		}
	}
	if opts.ClaudeStore == nil {
		return func() tea.Msg {
			return messages.ClaudeDispatchMsg{Err: errors.New("no repo root — cannot persist sessions")}
		}
	}

	req := claude.DispatchRequest{
		Env:     opts.ClaudeEnv,
		Store:   opts.ClaudeStore,
		Mode:    mode,
		Session: opts.TmuxSession,
		Prompt:  claude.Compose(mode, items),
	}
	primary := items[0]
	switch primary.Kind {
	case kindIssue:
		if primary.Issue != nil {
			req.Kind = kindIssue
			req.Ref = fmt.Sprintf("#%d", primary.Issue.IID)
			req.Title = primary.Issue.Title
		}
	case kindMR:
		if primary.MR != nil {
			req.Kind = kindMR
			req.Ref = fmt.Sprintf("!%d", primary.MR.IID)
			req.Title = primary.MR.Title
		}
	}
	if len(items) > 1 {
		req.Ref = fmt.Sprintf("%s (+%d)", req.Ref, len(items)-1)
	}

	return func() tea.Msg {
		var (
			res claude.Result
			err error
		)
		if mode == claude.ModeInteractive {
			res, err = claude.DispatchInteractive(req)
		} else {
			res, err = claude.DispatchOneShot(req)
		}
		if err != nil {
			return messages.ClaudeDispatchMsg{Err: err}
		}
		return messages.ClaudeDispatchMsg{Note: res.Note}
	}
}
