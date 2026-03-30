# lazydev — Recommendations for LLM-Assisted Debugging

## Context

lazydev is a working TUI for Docker + Kubernetes with live log tailing, filtering, search, and resource management. The goal is to evolve it into a tool that enables **LLM-assisted debugging workflows** — where an AI agent (like Claude Code) can analyze infrastructure logs and a human can guide the investigation.

The key insight from researching tools like Cursor Debug Mode, K8sGPT, Datadog Watchdog, and MCP servers: **the most impactful tools give LLMs structured access to runtime data, not just pretty displays for humans.**

---

## Recommended Improvements (Priority Order)

### Phase A: MCP Server Mode [HIGH IMPACT]

**Why**: This is the single most impactful feature. An MCP server lets Claude Code (or any LLM tool) directly query Docker/K8s logs without needing to read the TUI screen. The MCP protocol is becoming the standard for connecting AI agents to tools.

**What to build**: `lazydev mcp` starts an MCP server exposing these tools:

| MCP Tool            | Description                                                                            |
| ------------------- | -------------------------------------------------------------------------------------- |
| `list_resources`    | List all Docker containers + K8s pods with status                                      |
| `get_logs`          | Get logs from a specific container/pod (with tail, level filter, time range, regex)    |
| `search_logs`       | Search across ALL containers/pods for a pattern (returns matches with service context) |
| `get_errors`        | Get all ERROR/FATAL lines across all services in time order                            |
| `describe_resource` | Get container inspect or pod describe output                                           |
| `get_events`        | Get K8s events for a namespace or pod                                                  |
| `get_stats`         | Get CPU/memory stats for containers/pods                                               |

**How Claude Code would use it**:

```
Human: "The API is returning 500 errors, debug it"
Claude Code: [calls list_resources] → sees 12 services
Claude Code: [calls get_errors --last=5m] → finds errors in api-server and database
Claude Code: [calls get_logs --container=api-server --tail=200 --level=error] → sees connection refused to DB
Claude Code: [calls get_logs --container=postgres --tail=50] → sees OOM killed
Claude Code: "The postgres container was OOM killed, causing API 500s. Let me check the memory limits..."
```

**Files to create**:

- `cmd/lazydev/mcp.go` — MCP server entry point (flag: `lazydev mcp`)
- `internal/mcp/server.go` — MCP server implementation using stdio transport
- `internal/mcp/tools.go` — Tool definitions and handlers

---

### Phase B: CLI Mode with Structured Output [HIGH IMPACT]

**Why**: LLMs work best with structured text. A CLI mode that outputs JSON enables piping logs directly into LLM context. Also useful for scripting and CI/CD.

**Commands**:

```bash
lazydev logs api-server --tail=100 --level=error --json
lazydev logs --all --errors --last=5m --json
lazydev list --json
lazydev describe api-server --json
lazydev search "connection refused" --json
lazydev events --namespace=default --json
```

**JSON output format** (for logs):

```json
{
  "timestamp": "2026-03-30T10:15:32Z",
  "source": "docker",
  "container": "api-server",
  "level": "ERROR",
  "text": "connection refused to postgres:5432",
  "group": "my-app"
}
```

**Files to create**:

- `cmd/lazydev/cli.go` — CLI subcommands (using cobra or just flag parsing)
- `internal/output/json.go` — JSON formatters for all resource types

---

### Phase C: Log Export & Clipboard [MEDIUM IMPACT]

**Why**: The human needs a way to get log text OUT of the TUI and into Claude Code's context. Currently there's no way to copy/export.

**Features**:

- `y` — yank current log line to clipboard (using OSC52 escape sequence for terminal clipboard)
- `Y` — yank all visible/filtered lines to clipboard
- `e` — export current log view to file (`/tmp/lazydev-export-{timestamp}.log`)
- `E` — export as structured JSON

**Files to modify**:

- `internal/ui/components/logview.go` — add yank/export key handlers
- `internal/export/clipboard.go` — OSC52 clipboard support
- `internal/export/file.go` — file export

---

### Phase D: JSON Log Parsing [MEDIUM IMPACT]

**Why**: Most modern services output structured JSON logs. Currently lazydev displays them as raw strings. Parsing JSON would enable field-based filtering and much better readability.

**Features**:

- Auto-detect JSON log lines (starts with `{`)
- Parse and display as formatted key-value pairs with syntax highlighting
- Toggle between raw and parsed view (`J` key)
- Filter by JSON field: `/field:request_id=abc123`
- Extract common fields: timestamp, level, message, request_id, trace_id

**Files to modify**:

- `internal/log/highlight.go` — add JSON detection and parsing
- `internal/ui/components/logview.go` — add formatted JSON rendering mode

---

### Phase E: Cross-Service Correlation [MEDIUM IMPACT]

**Why**: Distributed system debugging requires tracing a request across services. If logs contain a shared ID (trace_id, request_id, correlation_id), we can filter all containers by that ID.

**Features**:

- `c` on a log line — extract IDs (trace_id, request_id, correlation_id patterns), search all containers for that ID
- Correlation view — shows matching log lines across all services in time order
- Auto-detect common ID patterns: UUID-like strings, `trace_id=X`, `X-Request-Id`, OpenTelemetry trace IDs

**Files to create**:

- `internal/log/correlate.go` — ID extraction and cross-service search
- `internal/ui/tabs/correlate.go` — correlation result view

---

### Phase F: Error Timeline View [LOW-MEDIUM IMPACT]

**Why**: When debugging, you often want "show me all errors across everything in the last 10 minutes." This is the All Logs tab but smarter.

**Features**:

- New "Errors" tab or mode within All Logs
- Pre-filtered to ERROR/FATAL/WARN across all sources
- Color-coded by service
- Timestamp-sorted
- Shows which service errored FIRST (root cause indicator)
- Time-bucketed view: "3 errors in api-server at 10:15, then 12 errors in worker at 10:16"

---

### Phase G: Bookmarks & Session Notes [LOW IMPACT]

**Why**: During debugging, you find interesting log lines and want to save them for later or share with an LLM. Bookmarks create a "trail" of investigation.

**Features**:

- `m` — bookmark current log line (toggle)
- `'` — jump between bookmarks
- `B` — view all bookmarks with context (3 lines before/after)
- Export bookmarks as markdown report

---

### Phase H: LLM Summary Integration (Optional) [EXPERIMENTAL]

**Why**: Direct LLM integration for log summarization. "What happened in the last 5 minutes?" Send log context to an LLM API and display the summary.

**Features**:

- `:summarize` command in command palette
- Sends recent logs (filtered) to configured LLM API
- Displays summary in detail pane
- Configurable API endpoint (Claude, OpenAI, local Ollama)

**Risk**: Adds API dependency, cost, latency. Better to prioritize MCP server (Phase A) which lets the LLM pull what it needs.

---

## Recommended Implementation Order

```
Phase A: MCP Server Mode        ← Do this first (biggest impact for LLM workflows)
Phase B: CLI Mode                ← Second (enables scripting + LLM piping)
Phase C: Log Export & Clipboard  ← Third (quick win for human-in-the-loop)
Phase D: JSON Log Parsing        ← Fourth (improves log readability)
Phase E: Cross-Service Correlation ← Fifth (distributed debugging)
Phase F: Error Timeline          ← Sixth (better error overview)
Phase G: Bookmarks               ← Seventh (investigation tracking)
Phase H: LLM Summary             ← Optional/experimental
```

## Key Architectural Insight

The most successful AI debugging tools follow this pattern:

1. **Structured data collection** — Parse logs into structured format (not raw strings)
2. **Programmatic access** — API/MCP/CLI for LLM to query (not just visual TUI)
3. **Cross-service correlation** — Link related events by trace ID / timestamp
4. **Human-in-the-loop** — Human can guide LLM with bookmarks, annotations, filtered views
5. **Export pipeline** — Easy to get data from tool into LLM context

lazydev already has #1 (partially) and #4. The biggest gaps are #2 (MCP/CLI) and #5 (export).

## Verification

- MCP Server: Configure Claude Code with the MCP server, ask it to debug a known issue
- CLI Mode: `lazydev logs --all --errors --json | head -20` produces valid JSON
- Export: Press `y` in TUI, paste in another terminal — correct log line
- JSON Parsing: View a container with JSON logs, see formatted output
