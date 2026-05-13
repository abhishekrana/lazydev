# Cut: saved views

Drop the saved-views feature in its entirety. The Query DSL on `/` already
covers the use case (filtering issues/MRs by `assignee:@me`, `label:bug`,
etc.); saved views were a convenience layer that bound common queries to
number keys `1`–`9` and palette commands `:save` / `:view` / `:del`.

## Rationale

Cuts ~250 lines and one entire package without weakening the core
"triage + dispatch to Claude" loop. Number keys revert to plain tab
switching, which is the legacy behavior most TUIs use. Users can still
narrow by retyping queries; muscle memory for `1`–`3` (Issues/MRs/Claude)
remains intact.

## What gets removed

### Files
- `internal/views/views.go` — YAML-backed store, atomic write, defaults
- `internal/views/views_test.go`

### Code references
- `pkg/messages/messages.go` — `ApplyViewMsg` type + section comment
- `internal/app/app.go` — `Views` field on `SharedState`, `views.Load` call,
  warning on load failure
- `cmd/lazydev/main.go` — drop `state.Views` arg to `NewRootModel`
- `internal/ui/root.go`:
  - `views` field on `RootModel`
  - `vs *views.Store` parameter on `NewRootModel`
  - Number-key view-recall branch (falls back to plain tab switch)
  - `applyView` method
  - `viewApplyDispatchMsg` internal envelope
  - `ApplyViewMsg` from the broadcast switch
  - `:save` / `:view` / `:del` palette commands in `executeCommand`
- `internal/ui/tabs/issues.go` — `messages.ApplyViewMsg` handler in `Update`
- `internal/ui/tabs/mergerequests.go` — same
- `internal/ui/components/help.go` — drop the two saved-view help rows

### Config / on-disk
- No config keys to drop (views path was hardcoded to `~/.config/lazydev/views.yaml`)
- `views.yaml` is left in place on user disks; lazydev simply ignores it

## What's kept

- Query DSL on `/` (untouched)
- Number keys `1`–`9` for tab switching (the fallback already in place,
  just becomes the only behavior)
- Command palette (`:tab`, `:help`, `:q` still work)

## Verification

After the cut:
- `go build ./...` clean
- `go vet ./...` clean
- `task lint` clean
- `go test ./...` — `internal/views` tests are removed; the rest pass
- Grep for `views\.` / `ApplyView` / `viewApplyDispatch` returns no hits
  outside `docs/`
