package cache

import (
	"context"
	"strings"
)

// SearchHit is a ranked result from FTS5 across both issues and MRs.
type SearchHit struct {
	Kind  string // "issue" | "mr"
	IID   int64
	Title string
	// Snippet is a short context excerpt with matched terms; HTML-free.
	Snippet string
	// Score is the bm25-like rank value (lower = better in FTS5).
	Score float64
}

// Search runs an FTS5 MATCH against title + body + notes for both kinds
// and returns the top `limit` hits ordered by FTS5 rank. The query is
// taken as-is; callers may pass FTS5 operators (`title:foo`, `bar*`).
func (s *Store) Search(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT
			search_fts.kind,
			search_fts.iid,
			snippet(search_fts, 2, '', '', '…', 12) AS snippet,
			bm25(search_fts) AS score
		 FROM search_fts
		 WHERE search_fts MATCH ?
		 ORDER BY score
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var hits []SearchHit
	for rows.Next() {
		var h SearchHit
		if err := rows.Scan(&h.Kind, &h.IID, &h.Snippet, &h.Score); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Populate titles in a second small query — cheaper than joining
	// search_fts to issues/mrs (FTS5 joins are awkward and rare).
	if err := s.fillTitles(ctx, hits); err != nil {
		return nil, err
	}
	return hits, nil
}

func (s *Store) fillTitles(ctx context.Context, hits []SearchHit) error {
	for i := range hits {
		var table string
		switch hits[i].Kind {
		case "issue":
			table = "issues"
		case "mr":
			table = "mrs"
		default:
			continue
		}
		var title string
		err := s.db.QueryRowContext(ctx,
			"SELECT title FROM "+table+" WHERE iid = ?",
			hits[i].IID,
		).Scan(&title)
		if err == nil {
			hits[i].Title = title
		}
	}
	return nil
}
