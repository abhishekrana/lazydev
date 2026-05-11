package tabs

// nothingToExport is the user-facing notification for export commands
// invoked with no marked or cursor item.
const nothingToExport = "nothing to export"

// noAIUserMsg is shown when an AI-handoff key (N or T) is pressed but
// cfg.GitLab.AIUser is unset.
const noAIUserMsg = "no ai_user configured"

// defaultEditor is the fallback when $EDITOR is unset.
const defaultEditor = "vim"

// Options bundles non-client settings shared by the Issues and MRs
// tabs (AI user for @ai resolution, Claude-export format and command).
// Carrying a single Options pointer keeps tab constructors stable as
// more knobs are added.
type Options struct {
	// AIUser is the GitLab username `@ai` resolves to.
	AIUser string
	// ExportFormat is either "claude-xml" or "markdown".
	ExportFormat string
	// LLMCommand is the shell-style invocation for Ctrl+Enter export
	// (e.g. "claude -p"). Empty string disables the pipe action.
	LLMCommand string
}
