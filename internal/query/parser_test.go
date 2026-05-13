package query

import (
	"testing"
	"time"
)

func TestParseStructured(t *testing.T) {
	env := Env{Me: "abhishek", AI: "claude-bot", Now: time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)}

	cases := []struct {
		in        string
		wantKind  string
		wantState string
		wantAssn  string
		wantAuth  string
		wantLbls  []string
		wantText  string
	}{
		{"assignee:@me label:bug state:open", "", "open", "abhishek", "", []string{"bug"}, ""},
		{"assignee:@ai", "", "", "claude-bot", "", nil, ""},
		{"author:alice", "", "", "", "alice", nil, ""},
		{"kind:mr state:merged", "mr", "merged", "", "", nil, ""},
		{"refresh token", "", "", "", "", nil, "refresh token"},
		{"label:bug label:backend mine", "", "", "", "", []string{"bug", "backend"}, "mine"},
		{"assignee:@none", "", "", "@none", "", nil, ""},
	}
	for _, c := range cases {
		got := Parse(c.in, env)
		if got.Kind != c.wantKind {
			t.Errorf("%q: Kind = %q want %q", c.in, got.Kind, c.wantKind)
		}
		if got.Filter.State != c.wantState {
			t.Errorf("%q: State = %q want %q", c.in, got.Filter.State, c.wantState)
		}
		if got.Filter.Assignee != c.wantAssn {
			t.Errorf("%q: Assignee = %q want %q", c.in, got.Filter.Assignee, c.wantAssn)
		}
		if got.Filter.Author != c.wantAuth {
			t.Errorf("%q: Author = %q want %q", c.in, got.Filter.Author, c.wantAuth)
		}
		if !sliceEqual(got.Filter.Labels, c.wantLbls) {
			t.Errorf("%q: Labels = %v want %v", c.in, got.Filter.Labels, c.wantLbls)
		}
		if got.Filter.Text != c.wantText {
			t.Errorf("%q: Text = %q want %q", c.in, got.Filter.Text, c.wantText)
		}
	}
}

func TestParseUpdated(t *testing.T) {
	now := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC)
	env := Env{Now: now}

	g := Parse("updated:>7d", env)
	want := now.Add(-7 * 24 * time.Hour)
	if !g.UpdatedAfter.Equal(want) {
		t.Errorf("updated:>7d = %v, want %v", g.UpdatedAfter, want)
	}

	g = Parse("updated:<30d", env)
	want = now.Add(-30 * 24 * time.Hour)
	if !g.UpdatedBefore.Equal(want) {
		t.Errorf("updated:<30d UpdatedBefore = %v, want %v", g.UpdatedBefore, want)
	}

	g = Parse("updated:=2026-05-01", env)
	wantStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !g.UpdatedAfter.Equal(wantStart) {
		t.Errorf("updated:=2026-05-01 UpdatedAfter = %v, want %v", g.UpdatedAfter, wantStart)
	}
}

func TestParseQuoted(t *testing.T) {
	g := Parse(`label:"area:auth" claude code`, Env{})
	if len(g.Filter.Labels) != 1 || g.Filter.Labels[0] != "area:auth" {
		t.Errorf("quoted label not preserved: %+v", g.Filter.Labels)
	}
	if g.Filter.Text != "claude code" {
		t.Errorf("fuzzy text wrong: %q", g.Filter.Text)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
