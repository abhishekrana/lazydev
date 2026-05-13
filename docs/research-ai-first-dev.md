# Research: AI-First Software Development Platform

**Author:** research pass for greenfield vision
**Date:** 2026-05-12
**Scope:** Three streams — (1) ticket/issue schema for AI consumption, (2) end-to-end AI dev workflow survey, (3) GitLab as a substrate for AI-first dev. Skeptical, sourced, and current as of May 2026.

---

## 1. Executive Summary

- **Spec-driven development (SDD) has effectively won the discourse battle in 2026.** Every major coding agent — GitHub Spec Kit, AWS Kiro, Tessl, Cursor, Claude Code, Google Antigravity — has shipped a flavor. The dominant artefact triad is `requirements.md` (EARS-style acceptance criteria), `design.md`, `tasks.md`, generally version-controlled in the repo. Issue trackers (Linear, GitLab, Jira) are increasingly seen as front-ends to those specs, not replacements for them.
- **Benchmarks have decoupled from reality.** SWE-bench Verified scores cluster at 80–94% for top frontier models (Claude Opus 4.7 at 87.6%, GPT-5.5 at 88.7%, "Claude Mythos Preview" at 93.9%), but SWE-bench Pro — designed to be harder to game and free of contamination — sits at 45–64%. OpenAI now [recommends Pro over Verified](https://www.morphllm.com/swe-bench-pro). Treat Verified scores as marketing.
- **Autonomy in the wild is mostly L2–L3.** Devin claims 67% PR merge rate (up from 34%) ([Cognition 2025 review](https://cognition.ai/blog/devin-annual-performance-review-2025)), but an [Answer.AI independent eval](https://www.sitepoint.com/devin-ai-engineers-production-realities/) ran 20 tasks with 14 failures, 3 successes, 3 unclear. Cursor reports [35% of internal PRs created by background agents](https://devops.com/cursor-cloud-agents-get-their-own-computers-and-35-of-internal-prs-to-prove-it/). The Pragmatic Engineer's 2026 [survey](https://newsletter.pragmaticengineer.com/p/ai-tooling-2026) of 900+ engineers finds 95% weekly AI use but only 55% regular agent use, and skeptics among non-agent-users are 2x more common.
- **The unit of work is shifting from "task" to "spec + scoped sandbox + verifier loop."** Plan-Implement-Verify (Coordinator-Implementor-Verifier) is the recurring multi-agent shape. Sandboxed VMs (E2B, Daytona, Modal, Vercel Sandboxes, Cloudflare) are now baseline infra — running an agent on a developer laptop is increasingly seen as legacy.
- **Context rot is the dominant systemic failure.** Performance degrades measurably past ~20–30 turns and ~30% context fill; success rate halves every doubling of task duration past 35 minutes ([Morph](https://www.morphllm.com/context-rot), [MindStudio](https://www.mindstudio.ai/blog/context-rot-ai-coding-agents-how-to-prevent)). The strongest mitigation is sub-agents with isolated contexts plus durable, file-based memory.
- **GitLab Duo Agent Platform went GA on Jan 15 2026** ([IR release](https://ir.gitlab.com/news/news-details/2026/GitLab-Announces-the-General-Availability-of-GitLab-Duo-Agent-Platform/default.aspx)). It offers "Flows" including Issue-to-MR, a foundational Planner agent and Security Analyst agent, MCP client, and webhook/event-driven custom flows. It's behind GitHub's Agent HQ in third-party-agent UX but ahead in DevSecOps data integration (CI, security findings, deploys all in one model).
- **Real opportunities exist around: (a) the spec ↔ issue ↔ diff traceability layer; (b) GitLab-native multi-agent orchestration that is more open than Duo Flows; (c) "agent cockpit" TUIs/UIs that treat issues, MRs, pipelines, and logs as a unified human+AI workspace; (d) verification/eval infra rather than yet-another code generator.**
- **The hype gap is widest at the autonomy ceiling.** Marketing claims L4–L5 ("autonomous engineer"), reality is mostly L2–L3 with strong human gates at PR. The product that wins is the one that makes the L2–L3 experience genuinely good — not the one that claims L5.

---

## 2. Ticket / Issue Schema for AI Consumption

### 2.1 The emerging artefact stack

By mid-2026 the field has converged on a layered artefact model, not a single ticket schema:

| Layer                          | Artefact                               | Owner                        | Lifetime     |
| ------------------------------ | -------------------------------------- | ---------------------------- | ------------ |
| Repo conventions               | `AGENTS.md`, `CLAUDE.md`               | Team                         | Long-lived   |
| Library / dependency knowledge | Tessl Spec Registry, DeepWiki          | Auto-generated, curated      | Long-lived   |
| Feature spec                   | `requirements.md` (EARS) + `design.md` | PM + Tech Lead + AI draft    | Per-feature  |
| Implementation plan            | `tasks.md` checklist                   | AI-generated, human-reviewed | Per-feature  |
| Tracker ticket                 | Issue/Work item (Linear, GitLab, Jira) | PM                           | Mirrors spec |
| Diff                           | MR / PR                                | Agent                        | Per-task     |

The tracker ticket is increasingly thin — a pointer to the spec, plus a status, assignment to an agent, and a thread for human comments. GitLab's Duo Developer Flow documentation makes this explicit: _"the Developer Flow only knows what you tell it or what is available in the context of the issue"_ ([GitLab Docs](https://docs.gitlab.com/user/duo_agent_platform/flows/issue_to_mr/)).

### 2.2 What context fields actually matter (synthesized from Devin, Cursor, GitHub Coding Agent, Spec Kit, Sweep, GitLab Duo Developer Flow)

A "ready-for-AI" ticket should carry, at minimum:

1. **Intent / user story** — Why this matters, who benefits. One sentence. The spec, not a vague title.
2. **Acceptance criteria in EARS notation** — `WHEN <trigger> THE SYSTEM SHALL <response>`, plus error cases. Each criterion must be testable. (See [Alistair Mavin's EARS guide](https://alistairmavin.com/ears/).)
3. **Affected files / entry points** — explicit paths or symbol pointers. Cursor, Aider, and Devin all see step-change improvements when paths are given vs. inferred.
4. **Related code / prior art** — links to similar past MRs, related issues, the function that does the analogous thing.
5. **Constraints** — perf budgets, dependencies that must not change, public API stability, framework/version pinning.
6. **Reproduction or context for bugs** — failing test name, stack trace, logs, env.
7. **Done signal** — what test(s) must pass, what command to run. This is the agent's success oracle and is non-optional for autonomous execution.
8. **Blast radius / rollback** — feature flag name if any, migration reversibility, who to notify.
9. **Owner / reviewer** — humans who must approve. Used by routing logic.
10. **Skip list / out-of-scope** — what the agent must NOT change. Scope creep is a known failure mode (Cognition explicitly calls it out).

Linear's Code Intelligence auto-drafts PR descriptions and tech specs from issues ([Linear Agent changelog](https://linear.app/changelog/2026-03-24-introducing-linear-agent)). GitLab Duo's Planner agent does similar decomposition. Notion, Shortcut, and Height each have their flavor, but none of them yet enforces structured machine-readable acceptance criteria — that's a gap.

### 2.3 EARS as the missing standard

EARS gives 4–5 patterns (Ubiquitous, State-driven `While`, Event-driven `When`, Optional `Where`, Unwanted `If…Then`) that collapse each requirement to a single testable claim. AWS Kiro's `requirements.md` uses EARS by default ([Kiro IDE review](https://www.cloudride.co.il/blog/kiro-ide-2026-review/)). GitHub Spec Kit is being [pushed to adopt EARS](https://github.com/github/spec-kit/issues/1356). For a greenfield product targeting agent consumption, EARS is the right default — it bridges natural language and test code with minimal training.

### 2.4 Right granularity

The consensus from Cognition, GitHub, and Tessl is:

- **Feature spec** is the primary unit (hours-to-days of work).
- **Task** is something one agent session can finish in <60 minutes — past that, context rot dominates.
- **Diff (MR/PR)** is the atomic review unit; one task should normally produce one MR.
- **Epic** is a human-curated grouping of features; AIs rarely operate at this level autonomously today.

Cognition: _"size tasks to something testable in hours, not days; provide context, constraints, and done criteria"_ ([Cognition perf review](https://cognition.ai/blog/devin-annual-performance-review-2025)).

### 2.5 What's missing in current trackers

- **Machine-readable acceptance criteria** (everything is markdown free-text).
- **First-class spec linkage** — issues link to specs in docs, but not as structured fields.
- **Blast radius / risk tagging** that a router can use.
- **Verifier hooks** — declarative bindings from "this AC" to "run this test".
- **Agent budget fields** — token budget, time budget, max retries.
- **Provenance** — when an agent edits an issue, what model, what tools were available, what prior state did it see.

---

## 3. The Autonomy Ladder (Concrete 2026 Map)

The Knight Institute paper and the HKUST/Tsinghua data-agent paper both use SAE-style L0–L5 scales ([arXiv 2506.12469](https://arxiv.org/html/2506.12469v1), [Knight Institute](https://knightcolumbia.org/content/levels-of-autonomy-for-ai-agents-1)). I'll use a coding-specific adaptation:

| Level  | Human role           | Definition                                                                                                           | Representative tools (May 2026)                                                                                                                                                        |
| ------ | -------------------- | -------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **L0** | Operator             | Pure manual; AI gives suggestions in-line.                                                                           | Plain editors with autocomplete                                                                                                                                                        |
| **L1** | Collaborator         | Inline AI completion. Human accepts char-by-char.                                                                    | GitHub Copilot autocomplete, Tabnine                                                                                                                                                   |
| **L2** | Consultant           | Chat / refactor / multi-file edits, human approves diffs in IDE.                                                     | Cursor inline Agent mode, Claude Code (interactive), Aider, Continue, Zed agent                                                                                                        |
| **L3** | Approver             | Asynchronous task delegation, agent works in sandbox, returns PR for human review. Tight scoping, well-defined task. | GitHub Copilot Coding Agent (GA), Cursor Cloud Agents (Feb 2026), OpenAI Codex Cloud, GitLab Duo Developer Flow / Issue-to-MR, Sweep, OpenHands Cloud, Augment, Factory, Tabnine Agent |
| **L4** | Observer (selective) | Long-horizon, multi-PR, multi-issue work with periodic human checkpoints; can self-recover from failures.            | Devin (claims L4, performs closer to high L3), Cognition Iris, Cursor "Build in Parallel", GitLab Duo Flows (multi-agent), some OpenHands deployments                                  |
| **L5** | Off-switch only      | Fully autonomous, owns roadmap-level work.                                                                           | **No production system exists at L5.** Research only.                                                                                                                                  |

### 3.1 Tool-by-tool quick map (what it owns, autonomy, queue, review gate)

- **Claude Code (Anthropic)** — Owns: any SDLC stage as a CLI/SDK harness. Autonomy: L2 interactive, L3 via Agent SDK + sub-agents. Queue: terminal session or programmatic. Review: human at every tool call by default; can be relaxed. The 2026 [Pragmatic Engineer survey](https://newsletter.pragmaticengineer.com/p/ai-tooling-2026) puts it at #1 "most loved" (46%) ahead of Cursor (19%) and Copilot (9%).
- **Cursor (Anysphere)** — IDE-first L2 with L3 [Cloud Agents](https://www.nxcode.io/resources/news/cursor-cloud-agents-virtual-machines-autonomous-coding-guide-2026) launched Feb 24 2026. Background agents run on isolated VMs, self-test, record video demos, open PRs. Internal claim: 35% of merged PRs from agents.
- **GitHub Copilot Coding Agent** — GA in 2026. Assign a GitHub issue, agent opens a draft PR, iterates in GitHub Actions runner, requests review. ([GH docs](https://docs.github.com/copilot/concepts/agents/coding-agent/about-coding-agent)). Now also part of "Agent HQ" — meta-orchestration over OpenAI/Anthropic/Google/xAI agents.
- **OpenAI Codex / Codex CLI** — Strong on Terminal-Bench 2.0 (82% with GPT-5.5). Codex Cloud at $200/mo runs sandboxed async PRs. CLI version is a Claude Code competitor.
- **Devin (Cognition)** — L3 marketed as L4. DeepWiki (auto-indexed codebase wiki), Devin Search, Playbooks (reusable system prompts). 67% PR merge rate self-reported; independent eval much weaker. Best for migrations, security patches, well-scoped repetitive work.
- **GitHub Spec Kit** — Open toolkit; templates and `/speckit.specify`, `/speckit.plan`, `/speckit.tasks` commands. Works across 30+ agents. Layer above the executor, not an executor itself.
- **AWS Kiro** — Spec-driven IDE on Bedrock. Auto-generates `requirements.md`/`design.md`/`tasks.md`. Has Agent Hooks (file-save, PR-open, repo-event triggers). Currently the most spec-rigorous IDE.
- **Tessl** — Pure SDD play. Tessl Framework + Spec Registry (10k+ library specs to prevent API hallucinations). [Series A in 2025](https://tessl.io/blog/announcing-our-series-a-for-ai-native-software-development/).
- **OpenHands (ex-OpenDevin, All-Hands AI)** — Open-source L3/L4. 72% SWE-bench Verified with Claude Sonnet 4.5 (best open-source). v1.6.0 in March 2026 with K8s + Planning Mode beta. $18.8M Series A.
- **SWE-agent (Princeton/Stanford)** — Research framework; defined the "Agent-Computer Interface" pattern.
- **Sweep** — GitHub-issue-to-PR, older school, configured via `sweep.yaml`.
- **Aider** — CLI pair programmer; popularized `CONVENTIONS.md` / `AGENTS.md` pattern.
- **Augment, Factory, Continue, Zed agent, Cline, Tabnine Agent** — L2 IDE assistants with various L3 add-ons.
- **Linear Agent (March 2026)** — Not a coder; an agent that lives in the issue tracker, drafts specs, summarizes Zendesk tickets, dispatches work to Cursor/Devin/Copilot. [Changelog](https://linear.app/changelog/2026-03-24-introducing-linear-agent).
- **GitLab Duo Agent Platform** — Foundational Planner + Security Analyst agents, Flows (Issue-to-MR, Code Review Flow), MCP client. Custom and external agents allowed.
- **Code review specialists** — CodeRabbit (multi-VCS, GitLab supported), Greptile (full-codebase indexing, 82% bug catch claim vs CodeRabbit's 44% — with more false positives), Ellipsis (PR summarization), Graphite Diamond (stacked PRs), Sourcery, Copilot Code Review.
- **Orchestration** — LangGraph (used inside GitLab Duo AI Gateway for checkpointing), Claude Agent SDK with sub-agents and Agent Teams, CrewAI, AutoGen Studio.

---

## 4. Workflow Patterns That Work

Recurring patterns across the survivors:

1. **Plan → Implement → Verify (PIV) outer loop.** Often a 3-agent split: a Planner produces a structured artefact (tasks.md); an Implementor (often parallel sub-agents) writes diffs; a Verifier independently checks against the original spec. The [Coordinator-Implementor-Verifier pattern](https://www.augmentcode.com/guides/coordinator-implementor-verifier) and the Anthropic April-2026 Planner/Generator/Evaluator harness are concrete instances.
2. **Sandboxed execution as the default.** E2B, Daytona, Modal, Cloudflare, Vercel sandboxes; Cursor Cloud Agents, Copilot Coding Agent, Codex Cloud. Firecracker / gVisor isolation. Removes the "agent broke my machine" failure mode and makes parallelism trivial.
3. **Human gate at the PR.** Even the most autonomous tools (Devin, Cursor Cloud, Copilot Agent) deliver a draft PR for human review. The PR is the universal review primitive. Tools that bypass it (auto-merge) are rare and risky.
4. **Spec-as-artefact, committed to the repo.** Versioning specs alongside code is what makes regeneration safe. Kiro and Spec Kit both enforce this. Tessl makes it explicit: "specs are long-term memory".
5. **Sub-agent isolation against context rot.** Long-running agents spawn task-scoped sub-agents whose contexts die when the task ends. Claude Code's sub-agents and Agent Teams are the canonical example.
6. **Tests as the agent's success oracle.** Done-criteria expressed as "this test passes" beats prose. EARS criteria → generated tests → green CI as the loop closer.
7. **MCP / standardized tool surface.** MCP went from niche to default in 2026. GitLab Duo, Linear, Claude Code, Cursor all consume MCP. Treat MCP as the _bus_, not as a feature.
8. **Repo-level convention files.** `AGENTS.md` is now a Linux Foundation–stewarded standard with 60k+ repos using it ([devtk.ai guide](https://devtk.ai/en/blog/what-is-agents-md-guide/)). Projects with detailed AGENTS.md report fewer agent-generated bugs.
9. **DeepWiki-style codebase indexing as background context.** Auto-generated wikis with architecture diagrams (Devin), or full-codebase embeddings (Greptile, Sourcegraph Cody) measurably improve PR quality.
10. **Asynchronous queues with progress visibility.** Background agents that surface progress (Cursor video demos, Copilot in-PR check-ins, GitLab Flow checkpoints via the Duo Workflow checkpointing API) reduce "trust me bro" anxiety.

---

## 5. Workflow Patterns That Fail

Skeptical view, grounded in primary failure-mode literature:

1. **Context rot.** U-shaped recall: high at start and end, 30% lower in the middle ([Morph](https://www.morphllm.com/context-rot)). Measurable degradation past 20–30 turns; success halves every doubling of duration past 35 minutes. Coding agents are uniquely vulnerable due to accumulative tool outputs and long horizons. Mitigation: sub-agents, file-based memory, planning frameworks that bound session scope.
2. **Hallucinated APIs / recursive hallucination.** Agent invents a function, compiler errors, agent writes a mock for the imaginary function instead of finding the real one ([medevel.com](https://medevel.com/the-infinite-loop-of-bad-decisions-why-your-ai-feedback-loop-is-broken-and-how-to-fix-it/)). Tessl's Spec Registry directly targets this.
3. **Doom loops / infinite tool loops.** Classic case: tool returns state that triggers the model to re-call the tool. TodoWrite returning the todo list back to the model → infinite update loop ([medium](https://medium.com/@dzianisv/vibe-engineering-agent-doom-loop-6158dff417be)). Solved by budget caps, idempotency markers, and not feeding tool-call state back as model input.
4. **Rabbit-holes on unexpected errors.** Devin's documented weakness: compounding "fixes" that worsen the code rather than stepping back. Independent eval: 14/20 tasks failed in this pattern.
5. **Scope creep.** Agent rewrites adjacent code, refactors what wasn't asked for. Worst in models that lean toward "helpful." Claude Code best practices explicitly warn: _"Claude Opus 4.5/4.6 have a tendency to overengineer."_ Out-of-scope fields and `AGENTS.md`-level constraints are the countermeasure.
6. **Review fatigue.** When agents produce N PRs/day, humans stop reviewing carefully. CodeRabbit/Greptile report meaningful FP rates (Greptile's 82% catch comes with 11 FPs vs CodeRabbit's 2). PR-fatigue is the structural risk of all L3+ adoption.
7. **Ambiguous tasks → wasted budget.** Devin's published failure mode: ambiguous requirements collide with autonomous execution. Where humans would ask, autonomous agents guess and burn tokens. The cost of an L4 agent guessing wrong is much higher than an L2 pairing.
8. **Monolithic codebases.** [Pragmatic Engineer survey](https://newsletter.pragmaticengineer.com/p/ai-tooling-2026): monoliths >~few-million LOC don't fit agent context. Big enterprise blocker.
9. **Defect rates 1.5–2× higher** than senior-authored code at equivalent complexity ([SitePoint summary](https://www.sitepoint.com/devin-ai-engineers-production-realities/)).
10. **Benchmark gaming.** SWE-bench Verified contamination (training-data leakage) → Pro is the new credible benchmark. Top model gap: 81% Verified vs 46% Pro ([Morph](https://www.morphllm.com/swe-bench-pro)).
11. **Procurement-driven tool choice.** 56% of large enterprises use Copilot despite being "less loved" — driven by Microsoft sales, not performance. The buyer is rarely the user.
12. **L4 marketing on L3 capability.** Three companies in the SitePoint piece abandoned Devin within a quarter because the autonomy promise collided with enterprise codebase reality.

---

## 6. GitLab as a Substrate for AI-First Dev

### 6.1 Duo state in May 2026

GitLab Duo Agent Platform reached GA on Jan 15 2026 ([IR release](https://ir.gitlab.com/news/news-details/2026/GitLab-Announces-the-General-Availability-of-GitLab-Duo-Agent-Platform/default.aspx)). Key capabilities ([docs](https://docs.gitlab.com/user/duo_agent_platform/)):

- **Agentic Chat** with multi-step reasoning across issues, MRs, pipelines, security findings.
- **Flows**: pre-defined orchestrations. Notably:
  - **Issue-to-MR Flow / Developer Flow** — mention `@duo-developer-<namespace>` or assign the Duo Developer service account. Produces draft MR + plan + research findings.
  - **Code Review Flow** — agentic review that scans changed files, pulls repo context, cross-references pipeline + security results, posts structured inline comments.
- **Foundational agents**: Planner (decompose epics → issues), Security Analyst (vuln triage + explanations).
- **Custom + external agents** allowed within governance.
- **MCP Client** — Duo can call out to Jira, Confluence, Slack, etc. (a notable concession that Duo doesn't own all the data.)
- **Duo Workflow Checkpointing APIs**: `POST/GET /ai/duo_workflows/workflows[/:id[/checkpoints]]` — LangGraph-shaped persistence ([MR !156872](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/156872)). Important: the GitLab Rails app is the source of truth for workflow state; the AI Gateway is stateless.

### 6.2 Pricing & self-hosting

- Duo Pro: +$19/user/month on Premium/Ultimate. Duo Enterprise: +$39/user/month.
- AI credit bundles: Premium $12/user/mo, Ultimate $29/user/mo. On-demand at $1/credit.
- Self-hosted models supported (GitLab 18.8+); offline customers need the Self-Hosted add-on.
- Trial: 30 days, 24 evaluation credits per user.

### 6.3 API/webhook surface relevant to agents

GitLab's APIs are very rich for agent builders, in many ways more so than GitHub's:

- **Webhooks** (project + group): push, issue events, MR events, pipeline events, deployment events, vulnerability events, work-item events, comment events. Filterable, deliverable to any HTTPS endpoint.
- **REST + GraphQL.** GraphQL exposes work items (the unified successor to issues/epics — required for new features like health status and the work-item hierarchy), MRs, pipelines, security findings.
- **Work Items API** — epics are now work items; migration guide published. The work-item hierarchy lets you model epic → feature → story → task → sub-task uniformly. Excellent fit for SDD layering.
- **Pipeline triggers + CI job tokens** — `CI_JOB_TOKEN` is per-job, short-lived, scoped, can post MR comments, trigger downstream pipelines, call the API. Makes CI a natural agent execution substrate.
- **Multi-project pipelines + child pipelines** — orchestration primitives that map onto multi-agent flows.
- **Service accounts** — Duo Developer uses one. Easy to issue agent-specific identities.
- **Iterations, milestones, labels** — first-class scheduling primitives that don't exist as cleanly in GitHub.

### 6.4 GitLab vs GitHub for AI agents

| Dimension                  | GitLab                                                       | GitHub                                                           |
| -------------------------- | ------------------------------------------------------------ | ---------------------------------------------------------------- |
| Native agent platform      | Duo Agent Platform (GA Jan 2026)                             | Copilot Coding Agent (GA), Agent HQ meta-orchestrator            |
| Third-party agent UX       | Less polished; MCP client added                              | Stronger — Agent HQ aggregates OpenAI/Anthropic/Google/xAI       |
| Data model integration     | **Strong** — CI, security, deploys, work items in one schema | Weaker — siloed across Actions, Advanced Security, separate apps |
| Webhook richness           | **Strong** (work items, pipelines, vulns)                    | Strong (issues, PRs, checks)                                     |
| CI as agent substrate      | **Strong** — CI is first-class                               | GitHub Actions is solid but less integrated into security/deploy |
| Spec-driven tooling        | None native; relies on agent tools                           | None native; GitHub Spec Kit is open-source side-project         |
| Self-hosted models         | Yes (18.8+)                                                  | Limited                                                          |
| Marketplace / ecosystem    | Smaller                                                      | **Much larger** — Apps, Marketplace, Actions catalog             |
| Open-source instinct       | Higher (open-core)                                           | Lower                                                            |
| Pure code-host UX velocity | Slower releases                                              | Faster                                                           |

**Net read:** GitLab is the better substrate when you want one platform's data model to be your context graph. GitHub is the better substrate when you want maximum third-party agent leverage and ecosystem reach. For an AI-first product, GitLab is _technically_ more capable as a backend and _less crowded_ as a product surface — which is both the opportunity and the risk.

---

## 7. Greenfield Opportunities

Where the market is thin, even in 2026:

1. **The unified human+AI cockpit for GitLab.** Linear-quality UX for issues + MRs + pipelines + logs, but with agents as first-class participants in the workspace, not as a bolted-on chatbot. (This is plausibly where lazydev itself sits — the TUI is the cockpit.) Duo is good at _automating_ GitLab; nobody is great at _visualizing_ the human+agent collaboration loop.
2. **Spec ↔ issue ↔ diff traceability.** Today specs live in `docs/specs/*.md`, issues live in trackers, MRs live in VCS. No first-class link with verifier hooks. A product that makes the AC↔test↔diff link explicit (and machine-readable in the work-item schema) would be a strong wedge.
3. **GitLab-native multi-agent orchestration that isn't Duo.** Duo Flows are useful but closed. A LangGraph- or Claude-Agent-SDK-based orchestrator that uses GitLab webhooks + work items + pipelines as primitives could be very compelling for teams that prefer open infrastructure and BYO model.
4. **Verification / eval infra rather than yet-another generator.** The market is saturated with generators and undersaturated with verifiers. A "Verifier-as-a-Service" that ingests spec + diff and outputs a structured verdict (with citations, regressions, missing AC) would be valuable even without generating code.
5. **Agent budget & cost governance.** Almost nobody is doing this well. Per-issue token caps, per-team monthly budgets, per-PR cost attribution, dashboards. As L3+ agents proliferate, this becomes table stakes.
6. **Context-window engineering as a product.** Today every team rolls their own AGENTS.md and prompt templates. A spec-aware context compiler that hydrates an agent run from work item + repo + PR + skill bundles + relevant past MRs is undersolved.
7. **A great CLI/TUI for AI-augmented day-to-day GitLab work.** Most GitLab Duo UX is in the web. Engineers live in terminals. Claude Code's CLI dominance shows the demand.
8. **Review-fatigue tooling.** Cross-PR aggregation, "trust gradient" routing (low-risk PRs auto-merge with passing tests + agent verifier sign-off; high-risk PRs surface to humans). Today this is hand-rolled per team.
9. **Domain-pack ecosystem.** Kiro's HealthOmics pack hints at the shape. Spec packs + verifier packs + skill packs per domain (Rails, Go, K8s, Terraform). Likely a marketplace play.
10. **Cross-org agent memory.** Most agent memory is per-session or per-repo. Org-level memory (architectural decisions, ADRs, past incidents) is rarely modeled. Companies like Cloudflare are starting to ship this — opportunity for a GitLab-native version.

---

## 8. Open Questions for the User

Before committing to a direction:

1. **Where on the autonomy ladder is the product targeting?** L2 (cockpit + inline pairing) is large and crowded but proven. L3 (issue → PR) is the hot middle. L4 (long-horizon multi-PR) is risky and largely marketing today.
2. **GitLab-only, or eventually GitHub too?** Duo's CI/security integration is a real moat — but GitHub is where the gravity is. Multi-VCS adds 2–3x cost.
3. **BYO model or opinionated stack?** BYO appeals to enterprise + open-source crowds; opinionated wins on UX. Hybrid is hard.
4. **Where does the spec live?** In the repo (Kiro/Spec Kit style), in the tracker (Linear style), or in a separate spec registry (Tessl style)? Each has implications for who edits, who owns, and how regeneration works.
5. **Who is the buyer?** IC engineer (Cursor, Claude Code), team lead (Linear, GitLab), VP eng (Devin, enterprise). Different products, different prices, different sales motions.
6. **What is the unique verifier?** Tests are universal; static analysis, type checks, perf budgets, security policies are differentiators. A defensible verifier story matters more than yet-another generator.
7. **How will you measure success?** "PRs merged" is gameable (low bar of merges). Acceptance-criteria-passed is harder but more honest. Defect-after-merge is the gold standard but slow.
8. **Open-source vs proprietary?** OpenHands' growth suggests OSS-with-cloud is viable; Tessl's Series A suggests pure-proprietary still raises. The defensibility is in workflow + data, not model.
9. **What's the lazydev → product evolution path?** Today lazydev is a unified TUI surface. The natural extensions are: agent participants in tabs (assignable, queryable), spec-aware issue views, verifier panes, budget displays. Each is incremental and testable.
10. **What do you give up by being GitLab-first?** Reach. The mitigation is being clearly the best GitLab agent platform — and that niche is open.

---

## 9. Sources (annotated)

### Primary product documentation

- [GitLab Duo Agent Platform docs](https://docs.gitlab.com/user/duo_agent_platform/) — definitive 2026 state of Duo's flows, agents, MCP integration.
- [GitLab Duo Developer Flow / Issue-to-MR](https://docs.gitlab.com/user/duo_agent_platform/flows/issue_to_mr/) — explicit context-field guidance for AI-consumable issues.
- [GitLab Duo Workflow Checkpointing API MR !156872](https://gitlab.com/gitlab-org/gitlab/-/merge_requests/156872) — how Duo persists agent state in Rails; LangGraph-shaped.
- [GitLab GA announcement](https://ir.gitlab.com/news/news-details/2026/GitLab-Announces-the-General-Availability-of-GitLab-Duo-Agent-Platform/default.aspx) — Jan 15 2026 IR release.
- [GitLab pricing 2026](https://about.gitlab.com/pricing/) — Duo add-ons cost structure.
- [GitHub Copilot Coding Agent docs](https://docs.github.com/copilot/concepts/agents/coding-agent/about-coding-agent) — the canonical L3 GitHub flow.
- [Cursor Cloud Agents (Feb 2026 launch)](https://www.nxcode.io/resources/news/cursor-cloud-agents-virtual-machines-autonomous-coding-guide-2026) — sandboxed VM agents.
- [Cursor: 35% of internal PRs are agent-created](https://devops.com/cursor-cloud-agents-get-their-own-computers-and-35-of-internal-prs-to-prove-it/) — rare hard number.
- [GitHub Spec Kit repo](https://github.com/github/spec-kit) — open SDD toolkit.
- [GitHub Spec Kit philosophy](https://github.com/github/spec-kit/blob/main/spec-driven.md) — the SDD argument: code serves specs.
- [Kiro docs (AWS)](https://kiro.dev/) — spec-driven IDE with Agent Hooks.
- [Tessl framework launch](https://tessl.io/blog/tessl-launches-spec-driven-framework-and-registry/) — spec registry to prevent API hallucinations.
- [Claude Code best practices](https://code.claude.com/docs/en/best-practices) — Anthropic's official prompt/context guidance.
- [Claude Agent SDK subagents](https://platform.claude.com/docs/en/agent-sdk/subagents) — orchestration primitives.
- [Linear Agent changelog (March 2026)](https://linear.app/changelog/2026-03-24-introducing-linear-agent) — Linear's agent-aware shift.
- [AGENTS.md spec](https://agents.md/) — Linux Foundation–stewarded convention file.

### Benchmarks & evaluations

- [SWE-bench Pro leaderboard](https://labs.scale.com/leaderboard/swe_bench_pro_public) — the credible benchmark.
- [Morph: why Pro beats Verified](https://www.morphllm.com/swe-bench-pro) — the contamination critique.
- [SWE-bench Verified](https://www.swebench.com/verified.html) — caveated leaderboard.
- [Terminal-Bench](https://www.morphllm.com/best-ai-coding-agents-2026) — Codex CLI / Claude Code rankings.

### Skeptical / contrarian

- [Cognition's Devin 2025 review](https://cognition.ai/blog/devin-annual-performance-review-2025) — 67% merge rate (vendor-reported); useful even if cherry-picked.
- [SitePoint: Devin in production](https://www.sitepoint.com/devin-ai-engineers-production-realities/) — three teams abandoned within a quarter; defect rates 1.5–2x.
- [Pragmatic Engineer AI Tooling 2026](https://newsletter.pragmaticengineer.com/p/ai-tooling-2026) — 95% weekly use, 55% agent use; skeptic correlation.
- [Steve Yegge on Pragmatic Engineer](https://newsletter.pragmaticengineer.com/p/from-ides-to-ai-agents-with-steve) — monolith blocker.
- [Vibe-engineering doom loop (Medium)](https://medium.com/@dzianisv/vibe-engineering-agent-doom-loop-6158dff417be) — concrete infinite-loop case.
- [IT Revolution: Vibe Coding Failure Patterns](https://itrevolution.com/wp-content/uploads/2025/02/Vibe-Coding-Failure-Patterns-and-Solution-Guide.pdf) — taxonomy.

### Context engineering / failure modes

- [Morph: Context Rot complete guide](https://www.morphllm.com/context-rot) — U-shape recall, 35-min half-life finding.
- [MindStudio: Context rot + sub-agents](https://www.mindstudio.ai/blog/context-rot-ai-coding-agents-sub-agents-fix) — mitigations.
- [Augment Code: Coordinator-Implementor-Verifier](https://www.augmentcode.com/guides/coordinator-implementor-verifier) — PIV pattern.
- [How Claude Code builds a system prompt (dbreunig)](https://www.dbreunig.com/2026/04/04/how-claude-code-builds-a-system-prompt.html) — concrete look at structure.

### Spec / acceptance criteria

- [Alistair Mavin's EARS guide](https://alistairmavin.com/ears/) — canonical EARS reference.
- [Augment: What is Spec-Driven Development](https://www.augmentcode.com/guides/what-is-spec-driven-development) — overview.
- [BCMS 2026 SDD guide](https://thebcms.com/blog/spec-driven-development) — full landscape.
- [github/spec-kit issue #1356: EARS adoption](https://github.com/github/spec-kit/issues/1356) — live discussion of standardizing EARS in Spec Kit.

### Autonomy frameworks

- [arXiv 2506.12469: Levels of Autonomy for AI Agents](https://arxiv.org/html/2506.12469v1) — academic L0–L5 framework.
- [Knight Institute: Levels of Autonomy](https://knightcolumbia.org/content/levels-of-autonomy-for-ai-agents-1) — policy framing.
- [TechLife: Data Agents L0–L5](https://techlife.blog/posts/data-agents/) — practitioner adaptation.

### Sandbox / infra

- [Blaxel: best code execution sandboxes 2026](https://blaxel.ai/blog/code-execution-sandboxes-for-ai-agents) — survey of E2B/Daytona/Modal/etc.
- [Cloudflare Project Think](https://blog.cloudflare.com/project-think/) — third-wave agent infra.

### Code review tooling

- [DeployHQ: CodeRabbit vs Copilot vs Sourcery vs Ellipsis](https://www.deployhq.com/blog/ai-code-review-tools-compared-coderabbit-copilot-sourcery-ellipsis) — practical comparison.
- [Greptile benchmarks](https://www.greptile.com/benchmarks) — vendor-reported numbers, take with salt.

---

_End of report._
