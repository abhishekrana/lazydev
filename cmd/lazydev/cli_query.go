package main

import (
	"context"

	"github.com/abhishek-rana/lazydev/internal/cache"
	"github.com/abhishek-rana/lazydev/internal/config"
	"github.com/abhishek-rana/lazydev/internal/query"
)

// parseQuery turns a user-provided DSL string into a cache.Filter,
// resolving @me / @ai the same way the TUI does. The Me username is
// pulled from the meta table where the TUI persists it on startup; if
// the cache has never been populated by a TUI run, @me silently resolves
// to "" — which the openCache check rejects earlier, so callers don't
// need to handle that case.
func parseQuery(ctx context.Context, store *cache.Store, cfg *config.Config, expr string) (kind string, filter cache.Filter) {
	me, _ := store.GetMeta(ctx, "gitlab_username")
	env := query.Env{
		Me: me,
		AI: cfg.GitLab.AIUser,
	}
	parsed := query.Parse(expr, env)
	filter = parsed.Filter
	if !parsed.UpdatedAfter.IsZero() {
		filter.UpdatedAfter = parsed.UpdatedAfter
	}
	if !parsed.UpdatedBefore.IsZero() {
		filter.UpdatedBefore = parsed.UpdatedBefore
	}
	return parsed.Kind, filter
}
