package claude

import (
	"fmt"
	"strings"

	"github.com/abhishek-rana/lazydev/internal/export"
)

// Compose returns a Claude-ready prompt string: an instruction header
// followed by the structured `<context>` XML produced by export. The
// header differs by mode so the agent's first move matches the user's
// intent — read+plan for interactive, propose-diff for one-shot.
func Compose(mode Mode, items []export.ExportItem) string {
	if len(items) == 0 {
		return ""
	}
	xml := export.BuildClaudeXML(items)
	header := instructionHeader(mode, items)
	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(xml)
	return b.String()
}

func instructionHeader(mode Mode, items []export.ExportItem) string {
	refs := refList(items)
	switch mode {
	case ModeOneShot:
		return fmt.Sprintf(
			"You are working on the following GitLab item(s): %s.\n"+
				"Read the structured context below, then produce a concrete "+
				"plan and any code changes you can make right now without "+
				"further input. Prefer small, reviewable diffs. If the task "+
				"is ambiguous, list the open questions instead of guessing.",
			refs,
		)
	default: // interactive
		return fmt.Sprintf(
			"You are pairing with a human engineer on the following GitLab "+
				"item(s): %s.\n"+
				"Start by acknowledging the task in one sentence and proposing "+
				"a short plan. Wait for confirmation before making code "+
				"changes. The structured context follows.",
			refs,
		)
	}
}

func refList(items []export.ExportItem) string {
	var refs []string
	for _, it := range items {
		switch it.Kind {
		case "issue":
			if it.Issue != nil {
				refs = append(refs, fmt.Sprintf("#%d", it.Issue.IID))
			}
		case "mr":
			if it.MR != nil {
				refs = append(refs, fmt.Sprintf("!%d", it.MR.IID))
			}
		}
	}
	if len(refs) == 0 {
		return "(unknown)"
	}
	return strings.Join(refs, ", ")
}
