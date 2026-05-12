package claude

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Mode is the dispatch mode of a session.
type Mode string

const (
	ModeInteractive Mode = "interactive"
	ModeOneShot     Mode = "one-shot"
)

// Status is the lifecycle state of a session record.
type Status string

const (
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Session is one dispatched Claude Code run, persisted to
// `.lazydev/sessions.json`. The record is opaque to Claude Code itself;
// we mint our own ID rather than parsing Claude's session UUIDs.
type Session struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"` // "issue" | "mr"
	Ref        string    `json:"ref"`  // "#421" or "!1024"
	Title      string    `json:"title,omitempty"`
	Mode       Mode      `json:"mode"`
	TmuxTarget string    `json:"tmux_target,omitempty"` // session:window for `tmux attach`
	PromptPath string    `json:"prompt_path,omitempty"` // path to the context tempfile
	LogPath    string    `json:"log_path,omitempty"`    // for one-shot runs
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExitNote   string    `json:"exit_note,omitempty"` // human-readable exit reason
}

// Store persists sessions to a JSON file. Concurrent access from a
// single lazydev process is serialized by the mutex; cross-process
// safety is best-effort — we re-read before each write to minimize the
// race window. If lazydev grows to multi-instance, swap to a lockfile.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore returns a Store bound to the given file path. The file is
// created lazily on first write.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path is the on-disk JSON path.
func (s *Store) Path() string { return s.path }

// List returns all sessions sorted newest-first by CreatedAt.
func (s *Store) List() ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readLocked()
}

// Add appends a new session.
func (s *Store) Add(sess Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.readLocked()
	if err != nil {
		return err
	}
	list = append(list, sess)
	return s.writeLocked(list)
}

// Update applies a mutator to the matching session. No-op if id absent.
func (s *Store) Update(id string, fn func(*Session)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.readLocked()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].ID == id {
			fn(&list[i])
			return s.writeLocked(list)
		}
	}
	return nil
}

// Delete removes the session with the given ID.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	list, err := s.readLocked()
	if err != nil {
		return err
	}
	out := list[:0]
	for _, sess := range list {
		if sess.ID != id {
			out = append(out, sess)
		}
	}
	return s.writeLocked(out)
}

func (s *Store) readLocked() ([]Session, error) {
	data, err := os.ReadFile(s.path) //nolint:gosec // user-owned file
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var list []Session
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *Store) writeLocked(list []Session) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:gosec // user-owned file
		return err
	}
	return os.Rename(tmp, s.path)
}

// NewID mints a short random identifier (8 hex chars).
func NewID() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
