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

type issueListJSON struct {
	IID         int64    `json:"iid"`
	Title       string   `json:"title"`
	State       string   `json:"state"`
	Status      string   `json:"status,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Assignees   []string `json:"assignees,omitempty"`
	Author      string   `json:"author,omitempty"`
	Milestone   string   `json:"milestone,omitempty"`
	Iteration   string   `json:"iteration,omitempty"`
	ParentIID   int64    `json:"parent_iid,omitempty"`
	ParentTitle string   `json:"parent_title,omitempty"`
	UpdatedAt   string   `json:"updated_at"`
	WebURL      string   `json:"web_url"`
}

type issueDetailJSON struct {
	Issue       issueFullJSON    `json:"issue"`
	Notes       []noteJSON       `json:"notes,omitempty"`
	RelatedMRs  []relatedMRJSON  `json:"related_mrs,omitempty"`
	LinkedItems []linkedItemJSON `json:"linked_items,omitempty"`
	ChildItems  []childItemJSON  `json:"child_items,omitempty"`
}

type issueFullJSON struct {
	IID            int64    `json:"iid"`
	ProjectID      int64    `json:"project_id"`
	Title          string   `json:"title"`
	State          string   `json:"state"`
	Status         string   `json:"status,omitempty"`
	Description    string   `json:"description,omitempty"`
	Labels         []string `json:"labels,omitempty"`
	Milestone      string   `json:"milestone,omitempty"`
	Iteration      string   `json:"iteration,omitempty"`
	IterationDates string   `json:"iteration_dates,omitempty"`
	Author         string   `json:"author,omitempty"`
	Assignees      []string `json:"assignees,omitempty"`
	ParentIID      int64    `json:"parent_iid,omitempty"`
	ParentTitle    string   `json:"parent_title,omitempty"`
	WebURL         string   `json:"web_url"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type noteJSON struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type relatedMRJSON struct {
	IID          int64  `json:"iid"`
	Title        string `json:"title"`
	State        string `json:"state"`
	SourceBranch string `json:"source_branch,omitempty"`
	WebURL       string `json:"web_url"`
}

type linkedItemJSON struct {
	IID      int64  `json:"iid"`
	Title    string `json:"title"`
	State    string `json:"state"`
	LinkType string `json:"link_type"`
	WebURL   string `json:"web_url"`
}

type childItemJSON struct {
	IID      int64  `json:"iid"`
	Title    string `json:"title"`
	State    string `json:"state"`
	ItemType string `json:"item_type,omitempty"`
	WebURL   string `json:"web_url"`
}

func cmdIssue(cfg *config.Config, args []string) int {
	if len(args) == 0 {
		printIssueHelp()
		return 2
	}
	switch args[0] {
	case "list":
		return cmdIssueList(cfg, args[1:])
	case "show":
		return cmdIssueShow(cfg, args[1:])
	case helpFlagShort, "-h", helpFlagLong:
		printIssueHelp()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "lazydev issue: unknown subcommand %q\n\n", args[0])
		printIssueHelp()
		return 2
	}
}

func printIssueHelp() {
	writeLines(os.Stderr,
		"Usage: lazydev issue <list|show> [flags]",
		"",
		"  list  --query \"DSL\" [--limit N] [--pretty]",
		"  show  <IID> [--with-notes] [--pretty]",
	)
}

func cmdIssueList(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("issue list", flag.ContinueOnError)
	q := fs.String("query", "", "lazydev query DSL, e.g. `assignee:@me state:open`")
	limit := fs.Int("limit", 0, "max rows (0 = no limit)")
	pretty := fs.Bool("pretty", false, "JSON array with indentation (default: NDJSON, one object per line)")
	usage(fs,
		"Usage: lazydev issue list [--query \"DSL\"] [--limit N] [--pretty]",
		"",
		"Query DSL: state:open|closed|all  assignee:@me|@ai|@none|<user>",
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
	if kind == "mr" {
		// `kind:mr` from the DSL explicitly excludes issues — empty list.
		if err := writeList[issueListJSON](nil, *pretty); err != nil {
			fail(err)
			return 1
		}
		return 0
	}
	if *limit > 0 {
		filter.Limit = *limit
	}

	issues, err := store.ListIssues(ctx, filter)
	if err != nil {
		fail(err)
		return 1
	}

	out := make([]issueListJSON, len(issues))
	for i, it := range issues {
		out[i] = toIssueListJSON(it)
	}
	if err := writeList(out, *pretty); err != nil {
		fail(err)
		return 1
	}
	return 0
}

func cmdIssueShow(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("issue show", flag.ContinueOnError)
	withNotes := fs.Bool("with-notes", false, "include the discussion thread (can be large)")
	pretty := fs.Bool("pretty", false, "indent the JSON")
	usage(fs,
		"Usage: lazydev issue show <IID> [--with-notes] [--pretty]",
		"",
		"Output shape:",
		`  {"issue":{…},"notes":[…],"related_mrs":[…],"linked_items":[…],"child_items":[…]}`,
		"Notes are omitted unless --with-notes is passed.",
	)
	if err := fs.Parse(reorderFlags(args, map[string]bool{"with-notes": true, "pretty": true})); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "lazydev issue show: exactly one IID required")
		fs.Usage()
		return 2
	}
	iid, err := strconv.ParseInt(fs.Arg(0), 10, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "lazydev issue show: invalid IID %q\n", fs.Arg(0))
		return 2
	}

	ctx := context.Background()
	store, err := openCache(ctx, cfg)
	if err != nil {
		fail(err)
		return 1
	}
	defer func() { _ = store.Close() }()

	issue, notes, related, err := store.GetIssue(ctx, iid)
	if err != nil {
		fail(err)
		return 1
	}
	if issue == nil {
		fmt.Fprintf(os.Stderr, "lazydev: issue #%d not in cache\n", iid)
		return 1
	}
	linked, err := store.ListLinkedItems(ctx, iid)
	if err != nil {
		fail(err)
		return 1
	}
	children, err := store.ListChildItems(ctx, iid)
	if err != nil {
		fail(err)
		return 1
	}

	out := issueDetailJSON{
		Issue:       toIssueFullJSON(*issue),
		RelatedMRs:  toRelatedMRsJSON(related),
		LinkedItems: toLinkedItemsJSON(linked),
		ChildItems:  toChildItemsJSON(children),
	}
	if *withNotes {
		out.Notes = toNotesJSON(notes)
	}
	if err := writeJSON(out, *pretty); err != nil {
		fail(err)
		return 1
	}
	return 0
}

// --- conversion helpers ---

func toIssueListJSON(it messages.GitLabIssue) issueListJSON {
	return issueListJSON{
		IID:         it.IID,
		Title:       it.Title,
		State:       it.State,
		Status:      it.Status,
		Labels:      it.Labels,
		Assignees:   it.Assignees,
		Author:      it.Author,
		Milestone:   it.Milestone,
		Iteration:   it.Iteration,
		ParentIID:   it.ParentIID,
		ParentTitle: it.ParentTitle,
		UpdatedAt:   it.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		WebURL:      it.WebURL,
	}
}

func toIssueFullJSON(it messages.GitLabIssue) issueFullJSON {
	return issueFullJSON{
		IID:            it.IID,
		ProjectID:      it.ProjectID,
		Title:          it.Title,
		State:          it.State,
		Status:         it.Status,
		Description:    it.Description,
		Labels:         it.Labels,
		Milestone:      it.Milestone,
		Iteration:      it.Iteration,
		IterationDates: it.IterationDates,
		Author:         it.Author,
		Assignees:      it.Assignees,
		ParentIID:      it.ParentIID,
		ParentTitle:    it.ParentTitle,
		WebURL:         it.WebURL,
		CreatedAt:      it.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      it.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

func toNotesJSON(notes []messages.GitLabNote) []noteJSON {
	out := make([]noteJSON, len(notes))
	for i, n := range notes {
		out[i] = noteJSON{
			Author:    n.Author,
			Body:      n.Body,
			CreatedAt: n.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
	}
	return out
}

func toRelatedMRsJSON(mrs []messages.GitLabIssueMR) []relatedMRJSON {
	out := make([]relatedMRJSON, len(mrs))
	for i, m := range mrs {
		out[i] = relatedMRJSON{
			IID:          m.IID,
			Title:        m.Title,
			State:        m.State,
			SourceBranch: m.SourceBranch,
			WebURL:       m.WebURL,
		}
	}
	return out
}

func toLinkedItemsJSON(items []messages.GitLabLinkedItem) []linkedItemJSON {
	out := make([]linkedItemJSON, len(items))
	for i, it := range items {
		out[i] = linkedItemJSON{
			IID:      it.IID,
			Title:    it.Title,
			State:    it.State,
			LinkType: it.LinkType,
			WebURL:   it.WebURL,
		}
	}
	return out
}

func toChildItemsJSON(items []messages.GitLabChildItem) []childItemJSON {
	out := make([]childItemJSON, len(items))
	for i, it := range items {
		out[i] = childItemJSON{
			IID:      it.IID,
			Title:    it.Title,
			State:    it.State,
			ItemType: it.ItemType,
			WebURL:   it.WebURL,
		}
	}
	return out
}
