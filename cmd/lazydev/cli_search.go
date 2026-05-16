package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/abhishek-rana/lazydev/internal/config"
)

type searchHitJSON struct {
	Kind    string  `json:"kind"`
	IID     int64   `json:"iid"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

func cmdSearch(cfg *config.Config, args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 20, "max number of hits")
	pretty := fs.Bool("pretty", false, "indent the output (default: compact JSON array)")
	usage(fs,
		"Usage: lazydev search [--limit N] [--pretty] <query>",
		"",
		"FTS5 full-text search across cached issues and MRs.",
		"Returns a JSON array of hits sorted by relevance.",
		"",
		"Output shape:",
		`  [{"kind":"issue|mr","iid":123,"title":"…","snippet":"…","score":-3.1}, …]`,
	)
	if err := fs.Parse(reorderFlags(args, map[string]bool{"pretty": true})); err != nil {
		return 2
	}
	q := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if q == "" {
		fmt.Fprintln(os.Stderr, "lazydev search: query required")
		fs.Usage()
		return 2
	}

	ctx := context.Background()
	store, err := openCache(ctx, cfg)
	if err != nil {
		fail(err)
		return 1
	}
	defer func() { _ = store.Close() }()

	hits, err := store.Search(ctx, q, *limit)
	if err != nil {
		fail(err)
		return 1
	}

	out := make([]searchHitJSON, len(hits))
	for i, h := range hits {
		out[i] = searchHitJSON{
			Kind:    h.Kind,
			IID:     h.IID,
			Title:   h.Title,
			Snippet: h.Snippet,
			Score:   h.Score,
		}
	}
	if err := writeJSON(out, *pretty); err != nil {
		fail(err)
		return 1
	}
	return 0
}
