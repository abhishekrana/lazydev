package cache

// schemaSQL is the full migration applied on Open. It is idempotent
// — every statement uses IF NOT EXISTS so a partially-initialized
// DB heals on the next start.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS issues (
    iid             INTEGER PRIMARY KEY,
    id              INTEGER NOT NULL DEFAULT 0,
    project_id      INTEGER NOT NULL DEFAULT 0,
    title           TEXT    NOT NULL DEFAULT '',
    description     TEXT    NOT NULL DEFAULT '',
    state           TEXT    NOT NULL DEFAULT 'opened',
    status          TEXT    NOT NULL DEFAULT '',
    labels          TEXT    NOT NULL DEFAULT '[]',
    milestone       TEXT    NOT NULL DEFAULT '',
    iteration_id    INTEGER NOT NULL DEFAULT 0,
    iteration       TEXT    NOT NULL DEFAULT '',
    iteration_dates TEXT    NOT NULL DEFAULT '',
    author          TEXT    NOT NULL DEFAULT '',
    assignees       TEXT    NOT NULL DEFAULT '[]',
    parent_iid      INTEGER NOT NULL DEFAULT 0,
    parent_title    TEXT    NOT NULL DEFAULT '',
    web_url         TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_issues_state    ON issues(state);
CREATE INDEX IF NOT EXISTS idx_issues_updated  ON issues(updated_at);

CREATE TABLE IF NOT EXISTS mrs (
    iid             INTEGER PRIMARY KEY,
    id              INTEGER NOT NULL DEFAULT 0,
    project_id      INTEGER NOT NULL DEFAULT 0,
    title           TEXT    NOT NULL DEFAULT '',
    description     TEXT    NOT NULL DEFAULT '',
    state           TEXT    NOT NULL DEFAULT 'opened',
    source_branch   TEXT    NOT NULL DEFAULT '',
    target_branch   TEXT    NOT NULL DEFAULT '',
    author          TEXT    NOT NULL DEFAULT '',
    assignees       TEXT    NOT NULL DEFAULT '[]',
    reviewers       TEXT    NOT NULL DEFAULT '[]',
    labels          TEXT    NOT NULL DEFAULT '[]',
    pipeline_status TEXT    NOT NULL DEFAULT '',
    changes_count   TEXT    NOT NULL DEFAULT '',
    web_url         TEXT    NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_mrs_state    ON mrs(state);
CREATE INDEX IF NOT EXISTS idx_mrs_updated  ON mrs(updated_at);

CREATE TABLE IF NOT EXISTS notes (
    id          INTEGER NOT NULL,
    parent_kind TEXT    NOT NULL,
    parent_iid  INTEGER NOT NULL,
    author      TEXT    NOT NULL DEFAULT '',
    body        TEXT    NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (parent_kind, parent_iid, id)
);
CREATE INDEX IF NOT EXISTS idx_notes_parent ON notes(parent_kind, parent_iid);

CREATE TABLE IF NOT EXISTS related_mrs (
    issue_iid     INTEGER NOT NULL,
    mr_iid        INTEGER NOT NULL,
    title         TEXT    NOT NULL DEFAULT '',
    state         TEXT    NOT NULL DEFAULT '',
    source_branch TEXT    NOT NULL DEFAULT '',
    web_url       TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (issue_iid, mr_iid)
);

CREATE TABLE IF NOT EXISTS linked_items (
    issue_iid  INTEGER NOT NULL,
    target_iid INTEGER NOT NULL,
    link_type  TEXT    NOT NULL DEFAULT '',
    title      TEXT    NOT NULL DEFAULT '',
    state      TEXT    NOT NULL DEFAULT '',
    web_url    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (issue_iid, target_iid)
);

CREATE TABLE IF NOT EXISTS child_items (
    parent_iid INTEGER NOT NULL,
    child_iid  INTEGER NOT NULL,
    title      TEXT    NOT NULL DEFAULT '',
    state      TEXT    NOT NULL DEFAULT '',
    item_type  TEXT    NOT NULL DEFAULT '',
    web_url    TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (parent_iid, child_iid)
);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE IF NOT EXISTS search_fts USING fts5(
    kind  UNINDEXED,
    iid   UNINDEXED,
    title,
    body,
    notes,
    tokenize = 'porter'
);
`
