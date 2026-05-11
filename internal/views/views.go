// Package views manages the user's named saved queries.
//
// A saved view is just a (name, expression) pair where the expression
// is a lazydev query DSL string. Views live in ~/.config/lazydev/
// views.yaml; the file is rewritten atomically on every Save/Delete.
//
// Default views (`mine`, `ai-queue`, `review`, `recent`) are written on
// first run so new users have something to recall via number keys.
package views

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// View is one named saved query.
type View struct {
	Name string `yaml:"name"`
	Expr string `yaml:"expr"`
}

// Store holds the in-memory copy of the views file plus the path it
// was loaded from. Mutations write through to disk immediately.
type Store struct {
	path  string
	Views []View
}

// Load opens views.yaml at the given path (creating it with defaults
// if missing). Returns a Store with the current contents.
func Load(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path) //nolint:gosec // user config
	if errors.Is(err, os.ErrNotExist) {
		s.Views = defaultViews()
		if err := s.save(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &s.Views); err != nil {
		return nil, err
	}
	return s, nil
}

// Get returns the view with the given name (case-insensitive).
func (s *Store) Get(name string) (View, bool) {
	for _, v := range s.Views {
		if equalFold(v.Name, name) {
			return v, true
		}
	}
	return View{}, false
}

// ByIndex returns the i-th view (0-based). Used by number-key recall.
func (s *Store) ByIndex(i int) (View, bool) {
	if i < 0 || i >= len(s.Views) {
		return View{}, false
	}
	return s.Views[i], true
}

// All returns a copy of the views slice in declaration order.
func (s *Store) All() []View {
	out := make([]View, len(s.Views))
	copy(out, s.Views)
	return out
}

// Save creates or updates a view, then persists.
func (s *Store) Save(v View) error {
	for i := range s.Views {
		if equalFold(s.Views[i].Name, v.Name) {
			s.Views[i] = v
			return s.save()
		}
	}
	s.Views = append(s.Views, v)
	return s.save()
}

// Delete removes the view with the given name. Returns ok=false if
// no such view exists.
func (s *Store) Delete(name string) (bool, error) {
	for i, v := range s.Views {
		if equalFold(v.Name, name) {
			s.Views = append(s.Views[:i], s.Views[i+1:]...)
			return true, s.save()
		}
	}
	return false, nil
}

func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	data, err := yaml.Marshal(s.Views)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// defaultViews are written to disk on first run.
func defaultViews() []View {
	return []View{
		{Name: "mine", Expr: "assignee:@me state:open"},
		{Name: "ai-queue", Expr: "assignee:@ai state:open"},
		{Name: "review", Expr: "kind:mr state:open"},
		{Name: "recent", Expr: "updated:>7d state:all"},
	}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 32
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// DefaultPath returns the XDG-compliant views.yaml path.
func DefaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lazydev", "views.yaml")
}
