# lazydk Roadmap

## Phase 1: Skeleton + Docker Logs [DONE]

1. Go module init, dependencies, entry point
2. Root Bubble Tea model with tab bar, status bar, resize handling
3. Lip Gloss theme and keybindings (vim + arrow keys)
4. Docker client + container listing
5. Sidebar component with Compose project grouping
6. Log stream manager + ring buffer (batched delivery: 50ms/100 lines)
7. Log viewport with live tail
8. **Milestone**: Run `lazydk`, see Docker containers, select one, see live logs

## Phase 2: Docker Actions + Polish

1. Collapsible groups in sidebar
2. Confirmation modal for destructive actions
3. Actions: restart, stop, remove (with modal confirmation)
4. Container inspect in detail pane (`D` key)
5. Split pane toggle (logs <-> details)
6. Error/success notifications in status bar

## Phase 3: Kubernetes Integration

1. client-go setup with kubeconfig discovery
2. Pod list, namespace grouping in sidebar
3. Pod/container log streaming (multi-container pod support)
4. Describe, get YAML, view events
5. Deployments, Services, StatefulSets in sidebar
6. Context switching between K8s clusters
7. **Milestone**: Full K8s tab — browse resources, view pod logs, describe resources

## Phase 4: Log Features

1. Regex and text search (`/` key) with match highlighting
2. Log level filtering (`f` key) — ERROR, WARN, INFO, DEBUG
3. Log level colorization (red, yellow, green, cyan)
4. Timestamp parsing and display toggle
5. Multi-source merger for All Logs tab (interleaved by timestamp)
6. Source labels in merged view (color-coded per container/pod)
7. **Milestone**: All Logs tab showing merged stream from Docker + K8s

## Phase 5: Dashboard Tab

1. Table component with sortable columns
2. Columns: Name, Type, Source (Docker/K8s), Status, Health, Restarts, CPU, Memory
3. Metrics server integration for K8s resource usage
4. Docker stats for container CPU/memory
5. Auto-refresh on configurable interval
6. Color-coded status indicators (green=healthy, red=crashed, yellow=pending)
7. **Milestone**: Dashboard showing all resources at a glance with live metrics

## Phase 6: Advanced Actions

1. Exec shell into container/pod (`x` key) via `tea.ExecProcess`
2. Port-forward (`p` key) — background goroutine, shown in status bar
3. Scale deployment (`S` key) — modal input for replica count
4. Delete with confirmation modal (`d` key)
5. Edit resource — opens `$EDITOR`, re-applies on save
6. K8s rollout restart
7. **Milestone**: Full management actions matching k9s + lazydocker capabilities

## Phase 7: Polish + Release

1. Help overlay (`?` key) showing all keybindings
2. Command palette (`:` mode) — `:exec`, `:scale 3`, `:filter ERROR`, etc.
3. Mouse support (click to select resources)
4. Config file hot-reload (watch with fsnotify)
5. Terminal resize handling edge cases
6. Minimum terminal size check (80x24)
7. goreleaser config for cross-platform binaries (Linux, macOS, Windows)
8. Homebrew formula
9. **Milestone**: v1.0 release

## Verification Plan

| Test | How |
|------|-----|
| Docker smoke test | `docker compose up` any project -> run `lazydk` -> containers appear -> select one -> live logs work |
| K8s test | Point at minikube/kind -> pods listed by namespace -> pod logs stream |
| Merged logs | All Logs tab -> logs from multiple sources interleaved by timestamp |
| Actions | Restart container from TUI -> verify restart -> logs resume automatically |
| Stress test | Tail high-volume log producer -> TUI stays responsive (batching works) |
| No backends | Neither Docker nor K8s available -> clear error message and exit |
| Single backend | Only Docker OR only K8s -> hide unavailable tab, show notice |
