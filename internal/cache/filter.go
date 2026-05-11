package cache

import (
	"strings"
	"time"
)

// Filter selects which issues / MRs to return from List* calls.
// A zero Filter means "open items, no other constraints".
//
// The Query DSL parser (see internal/query in later steps) translates a
// user expression like `assignee:@me label:bug state:open` into one of
// these. List* applies the structured constraints with SQL; the Text
// field is forwarded to FTS5 search for fuzzy matches.
type Filter struct {
	// State is one of "open" (default), "closed", "merged", or "all".
	// For issues, "merged" is treated as "all" since issues don't merge.
	State string

	// Assignee filters by exact username match. Use empty string for
	// "no assignee filter"; use "@none" for unassigned items.
	Assignee string

	// Author filters by exact author username.
	Author string

	// Labels filters items that have ALL of these labels.
	Labels []string

	// Text is a free-text fuzzy match against title + description.
	// Resolved through SQLite LIKE for now; query.Search() routes to
	// FTS5 for ranked cross-kind hits.
	Text string

	// UpdatedAfter, when non-zero, narrows results to updated_at > t.
	UpdatedAfter time.Time

	// UpdatedBefore, when non-zero, narrows results to updated_at < t.
	UpdatedBefore time.Time

	// Limit caps the number of returned rows. Zero means no limit.
	Limit int
}

func buildIssueQuery(f Filter) (string, []any) {
	return buildItemQuery("issues", issueCols, f)
}

func buildMRQuery(f Filter) (string, []any) {
	// MR state can be "merged" too; the helper handles both.
	return buildItemQuery("mrs", mrCols, f)
}

func buildItemQuery(table, cols string, f Filter) (string, []any) {
	var sb strings.Builder
	var args []any

	sb.WriteString("SELECT ")
	sb.WriteString(cols)
	sb.WriteString(" FROM ")
	sb.WriteString(table)
	sb.WriteString(" WHERE 1=1")

	switch strings.ToLower(f.State) {
	case "", "open":
		sb.WriteString(" AND state = 'opened'")
	case "closed":
		sb.WriteString(" AND state = 'closed'")
	case "merged":
		sb.WriteString(" AND state = 'merged'")
	case "all":
		// no constraint
	default:
		sb.WriteString(" AND state = ?")
		args = append(args, f.State)
	}

	if f.Assignee != "" {
		if f.Assignee == "@none" {
			sb.WriteString(" AND assignee = ''")
		} else {
			sb.WriteString(" AND assignee = ?")
			args = append(args, f.Assignee)
		}
	}
	if f.Author != "" {
		sb.WriteString(" AND author = ?")
		args = append(args, f.Author)
	}
	for _, lbl := range f.Labels {
		// Labels are stored as a JSON array; LIKE on the serialized
		// form is good enough for the typical case where label names
		// don't contain quotes. SQLite has no native array operators
		// in stock builds.
		sb.WriteString(` AND labels LIKE ?`)
		args = append(args, "%\""+lbl+"\"%")
	}
	if f.Text != "" {
		sb.WriteString(" AND (title LIKE ? OR description LIKE ?)")
		pat := "%" + f.Text + "%"
		args = append(args, pat, pat)
	}
	if !f.UpdatedAfter.IsZero() {
		sb.WriteString(" AND updated_at > ?")
		args = append(args, f.UpdatedAfter.Unix())
	}
	if !f.UpdatedBefore.IsZero() {
		sb.WriteString(" AND updated_at < ?")
		args = append(args, f.UpdatedBefore.Unix())
	}

	sb.WriteString(" ORDER BY updated_at DESC")
	if f.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, f.Limit)
	}
	return sb.String(), args
}
