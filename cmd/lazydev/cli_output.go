package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/abhishek-rana/lazydev/internal/cache"
	"github.com/abhishek-rana/lazydev/internal/config"
)

// helpFlag* are shared across subcommand dispatchers (silences goconst
// — these appear in main.go, cmdIssue, cmdMR).
const (
	helpFlagShort = "help"
	helpFlagLong  = "--help"
)

// openCache opens the cache at cfg.Cache.DBPath as a read-only consumer.
// It refuses to proceed when the cache hasn't been populated yet — the
// TUI is the only thing that talks to GitLab, so a missing/empty cache
// is a setup error, not a transient empty result.
func openCache(ctx context.Context, cfg *config.Config) (*cache.Store, error) {
	if _, err := os.Stat(cfg.Cache.DBPath); err != nil {
		return nil, fmt.Errorf("cache not found at %s — run `lazydev` once to populate it", cfg.Cache.DBPath)
	}
	store, err := cache.Open(ctx, cfg.Cache.DBPath)
	if err != nil {
		return nil, err
	}
	maxI, _ := store.MaxIssueUpdatedAt(ctx)
	maxM, _ := store.MaxMRUpdatedAt(ctx)
	if maxI.IsZero() && maxM.IsZero() {
		_ = store.Close()
		return nil, fmt.Errorf("cache is empty — run `lazydev` once to populate it from GitLab")
	}
	return store, nil
}

// writeJSON marshals v to stdout as a single JSON object/value.
// When pretty is true, the output is indented and ends with a newline.
func writeJSON(v any, pretty bool) error {
	enc := json.NewEncoder(os.Stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// writeList emits items either as a JSON array (when pretty) or as
// NDJSON (one object per line, the default). NDJSON lets shells
// pipeline through `jq -c` or `head` without buffering.
func writeList[T any](items []T, pretty bool) error {
	if pretty {
		return writeJSON(items, true)
	}
	enc := json.NewEncoder(os.Stdout)
	for _, it := range items {
		if err := enc.Encode(it); err != nil {
			return err
		}
	}
	return nil
}

// fail prints a one-line error to stderr.
func fail(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "lazydev: %v\n", err)
}

// writeLines writes each string as its own line to w, swallowing
// errors. Usage closures never need to surface a partial write.
func writeLines(w io.Writer, lines ...string) {
	for _, line := range lines {
		_, _ = io.WriteString(w, line+"\n")
	}
}

// usage installs a help-text printer on fs that writes lines via
// writeLines. Lets each subcommand declare its help as a slice of
// strings instead of repeating fmt.Fprintln/errcheck dances.
func usage(fs *flag.FlagSet, lines ...string) {
	fs.Usage = func() { writeLines(fs.Output(), lines...) }
}

// reorderFlags pre-processes args so positionals can appear before or
// after flags — `lazydev search login --limit 5` and
// `lazydev search --limit 5 login` both work. The stdlib flag package
// stops at the first non-flag, which is annoying for verbs that take
// a query string as the last positional.
//
// Pass the set of known boolean flag names (which don't consume the
// next token as a value). Any unknown -flag is assumed to take a value;
// callers that want strict parsing should use flag.NewFlagSet directly.
func reorderFlags(args []string, boolFlags map[string]bool) []string {
	var flagPart, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			// Everything after `--` is positional, by Unix convention.
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			name := strings.TrimLeft(a, "-")
			if strings.Contains(name, "=") {
				// --flag=value form, single token, never consumes next.
				flagPart = append(flagPart, a)
				continue
			}
			flagPart = append(flagPart, a)
			if !boolFlags[name] && i+1 < len(args) {
				flagPart = append(flagPart, args[i+1])
				i++
			}
			continue
		}
		positional = append(positional, a)
	}
	return append(flagPart, positional...)
}
