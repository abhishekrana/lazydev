// Package cache is the local SQLite mirror of GitLab issues and merge
// requests. The cache is the source of truth for reads — tabs render
// from it on startup with no network wait; a background Syncer keeps
// it fresh via updated_after polling.
package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (registered as "sqlite")

	"github.com/abhishek-rana/lazydev/pkg/messages"
)

// Store is the cache handle. Open returns one; Close releases it.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the cache DB at path. Missing directories
// are created. The returned Store is safe for concurrent use.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("cache dir: %w", err)
	}

	// WAL + busy_timeout give us cheap concurrent readers during the
	// sync goroutine's writes.
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// --- Issues ---

// UpsertIssues writes/replaces a batch of issues in one transaction
// and refreshes their search_fts rows.
func (s *Store) UpsertIssues(ctx context.Context, items []messages.GitLabIssue) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const upsert = `INSERT INTO issues (
		iid, id, project_id, title, description, state, labels, milestone,
		iteration_id, iteration, iteration_dates, author, assignee, web_url,
		created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(iid) DO UPDATE SET
		id              = excluded.id,
		project_id      = excluded.project_id,
		title           = excluded.title,
		description     = excluded.description,
		state           = excluded.state,
		labels          = excluded.labels,
		milestone       = excluded.milestone,
		iteration_id    = excluded.iteration_id,
		iteration       = excluded.iteration,
		iteration_dates = excluded.iteration_dates,
		author          = excluded.author,
		assignee        = excluded.assignee,
		web_url         = excluded.web_url,
		created_at      = excluded.created_at,
		updated_at      = excluded.updated_at`

	for _, it := range items {
		labels, _ := json.Marshal(it.Labels)
		if _, err := tx.ExecContext(ctx, upsert,
			it.IID, it.ID, it.ProjectID, it.Title, it.Description, it.State,
			string(labels), it.Milestone, it.IterationID, it.Iteration, it.IterationDates,
			it.Author, it.Assignee, it.WebURL,
			it.CreatedAt.Unix(), it.UpdatedAt.Unix(),
		); err != nil {
			return fmt.Errorf("upsert issue %d: %w", it.IID, err)
		}
		if err := refreshSearch(ctx, tx, "issue", it.IID, it.Title, it.Description); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListIssues returns issues filtered by the given Filter, ordered by
// updated_at DESC. Pass a zero-value Filter for "open issues, no other
// constraints".
func (s *Store) ListIssues(ctx context.Context, f Filter) ([]messages.GitLabIssue, error) {
	q, args := buildIssueQuery(f)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []messages.GitLabIssue
	for rows.Next() {
		issue, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

// GetIssue loads a single issue with its notes and related MRs.
func (s *Store) GetIssue(ctx context.Context, iid int64) (*messages.GitLabIssue, []messages.GitLabNote, []messages.GitLabIssueMR, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+issueCols+` FROM issues WHERE iid = ?`, iid)
	issue, err := scanIssue(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, err
	}

	notes, err := s.listNotes(ctx, "issue", iid)
	if err != nil {
		return nil, nil, nil, err
	}

	relRows, err := s.db.QueryContext(ctx,
		`SELECT mr_iid, title, state, source_branch, web_url FROM related_mrs WHERE issue_iid = ?`,
		iid,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	defer func() { _ = relRows.Close() }()

	var related []messages.GitLabIssueMR
	for relRows.Next() {
		var r messages.GitLabIssueMR
		if err := relRows.Scan(&r.IID, &r.Title, &r.State, &r.SourceBranch, &r.WebURL); err != nil {
			return nil, nil, nil, err
		}
		related = append(related, r)
	}
	return &issue, notes, related, relRows.Err()
}

// MaxIssueUpdatedAt returns the most-recent updated_at among cached
// issues, or zero time if empty.
func (s *Store) MaxIssueUpdatedAt(ctx context.Context) (time.Time, error) {
	var ts sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM issues`).Scan(&ts); err != nil {
		return time.Time{}, err
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return time.Unix(ts.Int64, 0), nil
}

// --- MRs ---

// UpsertMRs writes/replaces a batch of MRs in one transaction.
func (s *Store) UpsertMRs(ctx context.Context, items []messages.GitLabMR) error {
	if len(items) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	const upsert = `INSERT INTO mrs (
		iid, id, project_id, title, description, state,
		source_branch, target_branch, author, assignee, reviewers, labels,
		pipeline_status, changes_count, web_url,
		created_at, updated_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(iid) DO UPDATE SET
		id              = excluded.id,
		project_id      = excluded.project_id,
		title           = excluded.title,
		description     = excluded.description,
		state           = excluded.state,
		source_branch   = excluded.source_branch,
		target_branch   = excluded.target_branch,
		author          = excluded.author,
		assignee        = excluded.assignee,
		reviewers       = excluded.reviewers,
		labels          = excluded.labels,
		pipeline_status = excluded.pipeline_status,
		changes_count   = excluded.changes_count,
		web_url         = excluded.web_url,
		created_at      = excluded.created_at,
		updated_at      = excluded.updated_at`

	for _, m := range items {
		reviewers, _ := json.Marshal(m.Reviewers)
		labels, _ := json.Marshal(m.Labels)
		if _, err := tx.ExecContext(ctx, upsert,
			m.IID, m.ID, m.ProjectID, m.Title, m.Description, m.State,
			m.SourceBranch, m.TargetBranch, m.Author, m.Assignee,
			string(reviewers), string(labels),
			m.PipelineStatus, m.ChangesCount, m.WebURL,
			m.CreatedAt.Unix(), m.UpdatedAt.Unix(),
		); err != nil {
			return fmt.Errorf("upsert mr %d: %w", m.IID, err)
		}
		if err := refreshSearch(ctx, tx, "mr", m.IID, m.Title, m.Description); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListMRs returns MRs filtered by Filter, ordered by updated_at DESC.
func (s *Store) ListMRs(ctx context.Context, f Filter) ([]messages.GitLabMR, error) {
	q, args := buildMRQuery(f)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []messages.GitLabMR
	for rows.Next() {
		mr, err := scanMR(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mr)
	}
	return out, rows.Err()
}

// GetMR loads a single MR with its notes.
func (s *Store) GetMR(ctx context.Context, iid int64) (*messages.GitLabMR, []messages.GitLabNote, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+mrCols+` FROM mrs WHERE iid = ?`, iid)
	mr, err := scanMR(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	notes, err := s.listNotes(ctx, "mr", iid)
	return &mr, notes, err
}

// MaxMRUpdatedAt returns the most-recent updated_at among cached MRs.
func (s *Store) MaxMRUpdatedAt(ctx context.Context) (time.Time, error) {
	var ts sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM mrs`).Scan(&ts); err != nil {
		return time.Time{}, err
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return time.Unix(ts.Int64, 0), nil
}

// --- Notes ---

// UpsertNotes replaces the note set for a parent (kind, iid).
// The full set must be passed; existing notes for that parent are
// deleted first so that comment edits and deletions reflect.
func (s *Store) UpsertNotes(ctx context.Context, kind string, parentIID int64, notes []messages.GitLabNote) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE parent_kind = ? AND parent_iid = ?`, kind, parentIID); err != nil {
		return err
	}

	const ins = `INSERT INTO notes (id, parent_kind, parent_iid, author, body, created_at)
		VALUES (?,?,?,?,?,?)`
	for i, n := range notes {
		// Notes from GitLab carry an ID; we don't have it on the
		// messages.GitLabNote struct, so synthesize a stable per-parent
		// row index. Good enough for cache; not for cross-referencing
		// with the GitLab API.
		if _, err := tx.ExecContext(ctx, ins, int64(i+1), kind, parentIID, n.Author, n.Body, n.CreatedAt.Unix()); err != nil {
			return err
		}
	}

	// Re-aggregate notes content into the FTS row for that parent.
	var concat strings.Builder
	for _, n := range notes {
		concat.WriteString(n.Author)
		concat.WriteString(": ")
		concat.WriteString(n.Body)
		concat.WriteString("\n")
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE search_fts SET notes = ? WHERE kind = ? AND iid = ?`,
		concat.String(), kind, parentIID,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// UpsertRelatedMRs replaces the related-MR set for an issue.
func (s *Store) UpsertRelatedMRs(ctx context.Context, issueIID int64, mrs []messages.GitLabIssueMR) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM related_mrs WHERE issue_iid = ?`, issueIID); err != nil {
		return err
	}
	const ins = `INSERT INTO related_mrs (issue_iid, mr_iid, title, state, source_branch, web_url)
		VALUES (?,?,?,?,?,?)`
	for _, r := range mrs {
		if _, err := tx.ExecContext(ctx, ins, issueIID, r.IID, r.Title, r.State, r.SourceBranch, r.WebURL); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) listNotes(ctx context.Context, kind string, parentIID int64) ([]messages.GitLabNote, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT author, body, created_at FROM notes
		 WHERE parent_kind = ? AND parent_iid = ?
		 ORDER BY created_at ASC`,
		kind, parentIID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []messages.GitLabNote
	for rows.Next() {
		var n messages.GitLabNote
		var ts int64
		if err := rows.Scan(&n.Author, &n.Body, &ts); err != nil {
			return nil, err
		}
		n.CreatedAt = time.Unix(ts, 0)
		out = append(out, n)
	}
	return out, rows.Err()
}

// --- Meta ---

// SetMeta stores a key/value pair in the meta table.
func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO meta (key, value) VALUES (?,?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}

// GetMeta retrieves a meta value; returns "" if the key is missing.
func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// --- Janitor ---

// PruneOlderThan deletes closed/merged issues and MRs whose updated_at
// is older than the cutoff. Open items are kept regardless of age.
// Returns the number of rows removed across both tables.
func (s *Store) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	r1, err := tx.ExecContext(ctx,
		`DELETE FROM issues WHERE state = 'closed' AND updated_at < ?`,
		cutoff.Unix(),
	)
	if err != nil {
		return 0, err
	}
	n1, _ := r1.RowsAffected()

	r2, err := tx.ExecContext(ctx,
		`DELETE FROM mrs WHERE state IN ('closed', 'merged') AND updated_at < ?`,
		cutoff.Unix(),
	)
	if err != nil {
		return 0, err
	}
	n2, _ := r2.RowsAffected()

	// FTS rows for pruned items remain; cleanup is best-effort.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM search_fts WHERE (kind = 'issue' AND iid NOT IN (SELECT iid FROM issues))
		 OR (kind = 'mr' AND iid NOT IN (SELECT iid FROM mrs))`,
	); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n1 + n2, nil
}

// --- Scanning helpers ---

const issueCols = `iid, id, project_id, title, description, state, labels, milestone,
	iteration_id, iteration, iteration_dates, author, assignee, web_url,
	created_at, updated_at`

const mrCols = `iid, id, project_id, title, description, state,
	source_branch, target_branch, author, assignee, reviewers, labels,
	pipeline_status, changes_count, web_url, created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanIssue(r rowScanner) (messages.GitLabIssue, error) {
	var it messages.GitLabIssue
	var labels string
	var created, updated int64
	if err := r.Scan(
		&it.IID, &it.ID, &it.ProjectID, &it.Title, &it.Description, &it.State,
		&labels, &it.Milestone, &it.IterationID, &it.Iteration, &it.IterationDates,
		&it.Author, &it.Assignee, &it.WebURL, &created, &updated,
	); err != nil {
		return it, err
	}
	_ = json.Unmarshal([]byte(labels), &it.Labels)
	it.CreatedAt = time.Unix(created, 0)
	it.UpdatedAt = time.Unix(updated, 0)
	return it, nil
}

func scanMR(r rowScanner) (messages.GitLabMR, error) {
	var m messages.GitLabMR
	var reviewers, labels string
	var created, updated int64
	if err := r.Scan(
		&m.IID, &m.ID, &m.ProjectID, &m.Title, &m.Description, &m.State,
		&m.SourceBranch, &m.TargetBranch, &m.Author, &m.Assignee,
		&reviewers, &labels, &m.PipelineStatus, &m.ChangesCount, &m.WebURL,
		&created, &updated,
	); err != nil {
		return m, err
	}
	_ = json.Unmarshal([]byte(reviewers), &m.Reviewers)
	_ = json.Unmarshal([]byte(labels), &m.Labels)
	m.CreatedAt = time.Unix(created, 0)
	m.UpdatedAt = time.Unix(updated, 0)
	return m, nil
}

// refreshSearch upserts a search_fts row for the given (kind, iid).
// Notes content is preserved if present; this only refreshes title+body.
func refreshSearch(ctx context.Context, tx *sql.Tx, kind string, iid int64, title, body string) error {
	// Carry the existing notes blob across an FTS row replace.
	var notes string
	_ = tx.QueryRowContext(ctx, `SELECT notes FROM search_fts WHERE kind = ? AND iid = ?`, kind, iid).Scan(&notes)

	if _, err := tx.ExecContext(ctx, `DELETE FROM search_fts WHERE kind = ? AND iid = ?`, kind, iid); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO search_fts (kind, iid, title, body, notes) VALUES (?,?,?,?,?)`,
		kind, iid, title, body, notes,
	); err != nil {
		return err
	}
	return nil
}
