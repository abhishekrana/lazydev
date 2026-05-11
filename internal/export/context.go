// Package export — context.go builds Claude-ready prompt payloads
// from one or more cached GitLab issues / MRs.
//
// The default output format is XML (per Anthropic's prompt-engineering
// guidance on multi-document context). A markdown variant is also
// provided for piping to non-Claude tools (gh, aider, etc.).
package export

import (
	"fmt"
	"html"
	"os/exec"
	"strings"
	"time"

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// ExportItem is one issue or merge request bundled for export. Build
// callers populate either Issue or MR (not both); RelatedMRs / Notes
// are optional but encouraged for richer context.
type ExportItem struct {
	Kind       string // "issue" | "mr"
	Issue      *messages.GitLabIssue
	MR         *messages.GitLabMR
	Notes      []messages.GitLabNote
	RelatedMRs []messages.GitLabIssueMR
}

// BuildClaudeXML returns a `<context>…</context>` document wrapping
// every item in XML tags Anthropic models are trained on. The output
// is plain text — no surrounding markdown, ready to paste into a
// Claude prompt or pipe to `claude -p`.
func BuildClaudeXML(items []ExportItem) string {
	var b strings.Builder
	b.WriteString("<context>\n")
	for _, it := range items {
		switch it.Kind {
		case "issue":
			writeIssueXML(&b, it)
		case "mr":
			writeMRXML(&b, it)
		}
	}
	b.WriteString("</context>\n")
	return b.String()
}

// BuildMarkdown returns a human-readable markdown rendering. Useful
// for tempfile inspection or for tools that prefer markdown over XML.
func BuildMarkdown(items []ExportItem) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n---\n\n")
		}
		switch it.Kind {
		case "issue":
			writeIssueMD(&b, it)
		case "mr":
			writeMRMD(&b, it)
		}
	}
	return b.String()
}

// --- XML rendering ---

func writeIssueXML(b *strings.Builder, it ExportItem) {
	if it.Issue == nil {
		return
	}
	iss := it.Issue
	fmt.Fprintf(b, `  <issue id="#%d" url=%q state=%q assignee=%q author=%q updated=%q`,
		iss.IID, iss.WebURL, iss.State, iss.Assignee, iss.Author, iss.UpdatedAt.Format(time.RFC3339))
	if len(iss.Labels) > 0 {
		fmt.Fprintf(b, ` labels=%q`, strings.Join(iss.Labels, ","))
	}
	if iss.Milestone != "" {
		fmt.Fprintf(b, ` milestone=%q`, iss.Milestone)
	}
	if iss.Iteration != "" {
		fmt.Fprintf(b, ` iteration=%q`, iss.Iteration)
	}
	b.WriteString(">\n")

	fmt.Fprintf(b, "    <title>%s</title>\n", html.EscapeString(iss.Title))
	if iss.Description != "" {
		fmt.Fprintf(b, "    <body>%s</body>\n", html.EscapeString(strings.TrimSpace(iss.Description)))
	}
	writeNotesXML(b, it.Notes)
	writeRelatedMRsXML(b, it.RelatedMRs)
	b.WriteString("  </issue>\n")
}

func writeMRXML(b *strings.Builder, it ExportItem) {
	if it.MR == nil {
		return
	}
	mr := it.MR
	fmt.Fprintf(b, `  <mr id="!%d" url=%q state=%q assignee=%q author=%q source=%q target=%q updated=%q`,
		mr.IID, mr.WebURL, mr.State, mr.Assignee, mr.Author, mr.SourceBranch, mr.TargetBranch,
		mr.UpdatedAt.Format(time.RFC3339))
	if mr.PipelineStatus != "" {
		fmt.Fprintf(b, ` pipeline=%q`, mr.PipelineStatus)
	}
	if len(mr.Reviewers) > 0 {
		fmt.Fprintf(b, ` reviewers=%q`, strings.Join(mr.Reviewers, ","))
	}
	if len(mr.Labels) > 0 {
		fmt.Fprintf(b, ` labels=%q`, strings.Join(mr.Labels, ","))
	}
	b.WriteString(">\n")

	fmt.Fprintf(b, "    <title>%s</title>\n", html.EscapeString(mr.Title))
	if mr.Description != "" {
		fmt.Fprintf(b, "    <body>%s</body>\n", html.EscapeString(strings.TrimSpace(mr.Description)))
	}
	writeNotesXML(b, it.Notes)
	b.WriteString("  </mr>\n")
}

func writeNotesXML(b *strings.Builder, notes []messages.GitLabNote) {
	if len(notes) == 0 {
		return
	}
	b.WriteString("    <comments>\n")
	for _, n := range notes {
		fmt.Fprintf(b, "      <comment author=%q at=%q>%s</comment>\n",
			n.Author, n.CreatedAt.Format(time.RFC3339), html.EscapeString(strings.TrimSpace(n.Body)))
	}
	b.WriteString("    </comments>\n")
}

func writeRelatedMRsXML(b *strings.Builder, related []messages.GitLabIssueMR) {
	if len(related) == 0 {
		return
	}
	b.WriteString("    <related_mrs>\n")
	for _, r := range related {
		fmt.Fprintf(b, "      <mr id=\"!%d\" state=%q url=%q>%s</mr>\n",
			r.IID, r.State, r.WebURL, html.EscapeString(r.Title))
	}
	b.WriteString("    </related_mrs>\n")
}

// --- Markdown rendering ---

func writeIssueMD(b *strings.Builder, it ExportItem) {
	iss := it.Issue
	if iss == nil {
		return
	}
	fmt.Fprintf(b, "## ISSUE-%d: %s\n\n", iss.IID, iss.Title)
	fmt.Fprintf(b, "- State: %s\n- Author: %s\n", iss.State, iss.Author)
	if iss.Assignee != "" {
		fmt.Fprintf(b, "- Assignee: %s\n", iss.Assignee)
	}
	if len(iss.Labels) > 0 {
		fmt.Fprintf(b, "- Labels: %s\n", strings.Join(iss.Labels, ", "))
	}
	if iss.WebURL != "" {
		fmt.Fprintf(b, "- URL: %s\n", iss.WebURL)
	}
	if iss.Description != "" {
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(iss.Description))
		b.WriteString("\n")
	}
	writeNotesMD(b, it.Notes)
	writeRelatedMRsMD(b, it.RelatedMRs)
}

func writeMRMD(b *strings.Builder, it ExportItem) {
	mr := it.MR
	if mr == nil {
		return
	}
	fmt.Fprintf(b, "## MR-%d: %s\n\n", mr.IID, mr.Title)
	fmt.Fprintf(b, "- State: %s\n- Author: %s\n- Branch: %s → %s\n",
		mr.State, mr.Author, mr.SourceBranch, mr.TargetBranch)
	if mr.PipelineStatus != "" {
		fmt.Fprintf(b, "- Pipeline: %s\n", mr.PipelineStatus)
	}
	if len(mr.Reviewers) > 0 {
		fmt.Fprintf(b, "- Reviewers: %s\n", strings.Join(mr.Reviewers, ", "))
	}
	if mr.WebURL != "" {
		fmt.Fprintf(b, "- URL: %s\n", mr.WebURL)
	}
	if mr.Description != "" {
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(mr.Description))
		b.WriteString("\n")
	}
	writeNotesMD(b, it.Notes)
}

func writeNotesMD(b *strings.Builder, notes []messages.GitLabNote) {
	if len(notes) == 0 {
		return
	}
	b.WriteString("\n### Comments\n\n")
	for _, n := range notes {
		fmt.Fprintf(b, "**@%s** (%s):\n%s\n\n", n.Author, n.CreatedAt.Format("2006-01-02 15:04"), strings.TrimSpace(n.Body))
	}
}

func writeRelatedMRsMD(b *strings.Builder, related []messages.GitLabIssueMR) {
	if len(related) == 0 {
		return
	}
	b.WriteString("\n### Related MRs\n\n")
	for _, r := range related {
		fmt.Fprintf(b, "- !%d [%s] %s — %s\n", r.IID, r.State, r.Title, r.WebURL)
	}
}

// --- Delivery channels ---

// CopyClipboard writes the OSC52 escape sequence directly to /dev/tty,
// bypassing the Bubble Tea screen buffer. Works over SSH/tmux when the
// host terminal honors OSC52. Returns an error if /dev/tty isn't
// writable (e.g. running non-interactively).
func CopyClipboard(content string) error {
	// On Linux/macOS /dev/tty is always the controlling terminal even
	// while the program owns stdout. Windows TUIs don't have this
	// path; we'd need a different mechanism there.
	f, err := openTTY()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = f.WriteString(ToClipboardOSC52(content))
	return err
}

// PipeToCommand runs the user's configured llm_command with content
// on stdin. Returns combined stdout+stderr. Useful for `claude -p` or
// equivalent CLI invocations.
//
// The command string is split on whitespace (no shell parsing), so
// quote-handling features of $SHELL are not available. Users who need
// pipelines or env-var interpolation should wrap their command in
// `bash -c '…'` in config.
func PipeToCommand(cmdline, content string) ([]byte, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty llm_command")
	}
	c := exec.Command(parts[0], parts[1:]...) //nolint:gosec,noctx // user-configured command, intentional
	c.Stdin = strings.NewReader(content)
	return c.CombinedOutput()
}
