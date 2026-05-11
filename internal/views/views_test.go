package views

import (
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "views.yaml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(s.Views) < 3 {
		t.Fatalf("expected at least 3 default views, got %d", len(s.Views))
	}
	if _, ok := s.Get("mine"); !ok {
		t.Fatalf("default 'mine' view missing")
	}

	// Reload from same file → still has defaults.
	s2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(s2.Views) != len(s.Views) {
		t.Fatalf("reload mismatch: %d != %d", len(s2.Views), len(s.Views))
	}
}

func TestSaveAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "views.yaml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	before := len(s.Views)

	if err := s.Save(View{Name: "urgent", Expr: "label:urgent state:open"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(s.Views) != before+1 {
		t.Fatalf("expected %d views after save, got %d", before+1, len(s.Views))
	}

	// Re-saving the same name updates rather than appends.
	if err := s.Save(View{Name: "URGENT", Expr: "label:p0 state:open"}); err != nil {
		t.Fatalf("Save update: %v", err)
	}
	if len(s.Views) != before+1 {
		t.Fatalf("update should not append; got %d views", len(s.Views))
	}
	v, _ := s.Get("urgent")
	if v.Expr != "label:p0 state:open" {
		t.Fatalf("update did not change expr: %q", v.Expr)
	}

	// Persistence roundtrip.
	s2, _ := Load(path)
	if _, ok := s2.Get("urgent"); !ok {
		t.Fatalf("urgent missing after reload")
	}

	// Delete.
	ok, err := s.Delete("urgent")
	if err != nil || !ok {
		t.Fatalf("Delete: ok=%v err=%v", ok, err)
	}
	if _, ok := s.Get("urgent"); ok {
		t.Fatalf("urgent still present after delete")
	}
}

func TestByIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "views.yaml")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v, ok := s.ByIndex(0); !ok || v.Name == "" {
		t.Fatalf("ByIndex(0) failed: ok=%v v=%+v", ok, v)
	}
	if _, ok := s.ByIndex(999); ok {
		t.Fatalf("ByIndex(999) should return false")
	}
}
