package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/abhishek-rana/lazydev/internal/config"
	"github.com/abhishek-rana/lazydev/pkg/messages"
)

type mrListJSON struct {
	IID            int64    `json:"iid"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	SourceBranch   string   `json:"source_branch,omitempty"`
	TargetBranch   string   `json:"target_branch,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Assignees      []string `json:"assignees,omitempty"`
	Reviewers      []string `json:"reviewers,omitempty"`
	Author         string   `json:"author,omitempty"`
	PipelineStatus string   `json:"pipeline_status,omitempty"`
	ChangesCount   string   `json:"changes_count,omitempty"`
	UpdatedAt      string   `json:"updated_at"`
	WebURL         string   `json:"web_url"`
}

type mrDetailJSON struct {
	MR    mrFullJSON `json:"mr"`
	Notes []noteJSON `json:"notes,omitempty"`
}

type mrFullJSON struct {
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	Description    string   `json:"description,omitempty"`
	SourceBranch   string   `json:"source_branch,omitempty"`
	TargetBranch   string   `json:"target_branch,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Assignees      []string `json:"assignees,omitempty"`
	Reviewers      []string `json:"reviewers,omitempty"`
	Author         string   `json:"author,omitempty"`
	PipelineStatus string   `json:"pipeline_status,omitempty"`
	ChangesCount   string   `json:"changes_count,omitempty"`
	WebURL         string   `json:"web_url"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

func cmdMR(cfg *config.Config, args []string) int {
	if len(args) == 0 {
		printMRHelp()
		return 2
	}
	switch args[0] {
	case "list":
		return cmdMRList(cfg, args[1:])
	case "show":
		return cmdMRShow(cfg, args[1:])
	case helpFlagShort, "-h", helpFlagLong:
		printMRHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "lazydev mr: unknown subcommand %q\n\n", args[0])
		printMRHelp()
		return 2
	}
}

func printMRHelp() {
	writeLines(os.Stderr,
		"Usage: lazydev mr <list|show> [flags]",
		"",
		"  list  --query \"DSL\" [--limit N] [--pretty]",
		"  show  <IID> [--with-notes] [--pretty]",
	)
}

func cmdMRList(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("mr list", flag.ContinueOnError)
	q := fs.String("query", "", "lazydev query DSL, e.g. `state:open assignee:@me`")
	limit := fs.Int("limit", 0, "max rows (0 = no limit)")
	pretty := fs.Bool("pretty", false, "JSON array with indentation (default: NDJSON)")
	usage(fs,
		"Usage: lazydev mr list [--query \"DSL\"] [--limit N] [--pretty]",
		"",
		"Query DSL: state:open|closed|merged|all  assignee:@me|@ai|@none|<user>",
		"           author:<user>  label:foo  updated:>7d  bare-fuzzy-text",
		"Default output is NDJSON; pass --pretty for a JSON array.",
	)
	if err := fs.Parse(reorderFlags(args, map[string]bool{"pretty": true})); err != nil {
		return 2
	}

	ctx := context.Background()
	store, err := openCache(ctx, cfg)
	if err != nil {
		fail(err)
		return 1
	}
	defer func() { _ = store.Close() }()

	kind, filter := parseQuery(ctx, store, cfg, *q)
	if kind == "issue" {
		if err := writeList[mrListJSON](nil, *pretty); err != nil {
			fail(err)
			return 1
		}
		return 0
	}
	if *limit > 0 {
		filter.Limit = *limit
	}

	mrs, err := store.ListMRs(ctx, filter)
	if err != nil {
		fail(err)
		return 1
	}

	out := make([]mrListJSON, len(mrs))
	for i, m := range mrs {
		out[i] = toMRListJSON(m)
	}
	if err := writeList(out, *pretty); err != nil {
		fail(err)
		return 1
	}
	return 0
}

func cmdMRShow(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("mr show", flag.ContinueOnError)
	withNotes := fs.Bool("with-notes", false, "include the discussion thread")
	pretty := fs.Bool("pretty", false, "indent the JSON")
	usage(fs,
		"Usage: lazydev mr show <IID> [--with-notes] [--pretty]",
		"",
		"Output shape: {\"mr\":{…},\"notes\":[…]}",
		"Notes are omitted unless --with-notes is passed.",
	)
	if err := fs.Parse(reorderFlags(args, map[string]bool{"with-notes": true, "pretty": true})); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "lazydev mr show: exactly one IID required")
		fs.Usage()
		return 2
	}
	iid, err := strconv.ParseInt(fs.Arg(0), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydev mr show: invalid IID %q\n", fs.Arg(0))
		return 2
	}

	ctx := context.Background()
	store, err := openCache(ctx, cfg)
	if err != nil {
		fail(err)
		return 1
	}
	defer func() { _ = store.Close() }()

	mr, notes, err := store.GetMR(ctx, iid)
	if err != nil {
		fail(err)
		return 1
	}
	if mr == nil {
		fmt.Fprintf(os.Stderr, "lazydev: MR !%d not in cache\n", iid)
		return 1
	}

	out := mrDetailJSON{MR: toMRFullJSON(*mr)}
	if *withNotes {
		out.Notes = toNotesJSON(notes)
	}
	if err := writeJSON(out, *pretty); err != nil {
		fail(err)
		return 1
	}
	return 0
}

func toMRListJSON(m messages.GitLabMR) mrListJSON {
	return mrListJSON{
		IID:            m.IID,
		Title:          m.Title,
		State:          m.State,
		SourceBranch:   m.SourceBranch,
		TargetBranch:   m.TargetBranch,
		Labels:         m.Labels,
		Assignees:      m.Assignees,
		Reviewers:      m.Reviewers,
		Author:         m.Author,
		PipelineStatus: m.PipelineStatus,
		ChangesCount:   m.ChangesCount,
		UpdatedAt:      m.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		WebURL:         m.WebURL,
	}
}

func toMRFullJSON(m messages.GitLabMR) mrFullJSON {
	return mrFullJSON{
		IID:            m.IID,
		ProjectID:      m.ProjectID,
		Title:          m.Title,
		State:          m.State,
		Description:    m.Description,
		SourceBranch:   m.SourceBranch,
		TargetBranch:   m.TargetBranch,
		Labels:         m.Labels,
		Assignees:      m.Assignees,
		Reviewers:      m.Reviewers,
		Author:         m.Author,
		PipelineStatus: m.PipelineStatus,
		ChangesCount:   m.ChangesCount,
		WebURL:         m.WebURL,
		CreatedAt:      m.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      m.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
