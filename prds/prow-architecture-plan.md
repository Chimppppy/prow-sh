# Prow — Technical Architecture & Build Plan

> **Status:** Draft v0.4 — Living Document
> **Owner:** Jonah DaCosta
> **Last updated:** April 26, 2026
> **Document source:** Plan-mode architecture session, Parts 1–9
> **v0.4 changes:** Split deployment into two binaries — `prow` (CLI, analyst-installed) and `prowd` (server, engineer-deployed). Web UI is a `prowd`-only feature, not embedded in the analyst CLI. Affects Parts 2, 7, 8.

---

## Table of Contents

- [Decision Log](#decision-log)
- [Open Questions](#open-questions)
- [Part 1 — Product Foundations & Architectural Principles](#part-1--product-foundations--architectural-principles)
- [Part 2 — System Architecture (Service Topology)](#part-2--system-architecture-service-topology)
- [Part 3 — Data Architecture](#part-3--data-architecture)
- [Part 4 — The Connector Framework](#part-4--the-connector-framework)
- [Part 5 — The Prow Common Schema (PCS)](#part-5--the-prow-common-schema-pcs)
- [Part 6 — The Agent Runtime + Consent State Machine](#part-6--the-agent-runtime--consent-state-machine)
- [Part 7 — Deployment & Distribution](#part-7--deployment--distribution)
- [Part 8 — Phasing & Roadmap](#part-8--phasing--roadmap)
- [Part 9 — Risks I'm Tracking](#part-9--risks-im-tracking)

---

## Decision Log

All architectural decisions resolved in the planning session. **25 of 25 raised DPs locked.**

| ID | Decision | Resolution |
|---|---|---|
| DP-1.1 | BYOLLM scope | OpenAI, Anthropic, Azure OpenAI, AWS Bedrock, Ollama, vLLM, plus generic OpenAI-compatible endpoint adapter |
| DP-1.2 | Open-source license | **Deferred.** Architecture is license-neutral; positioning decision can be made later. Default recommendation: BSL with 4-year Apache 2.0 conversion clause (Sentry's playbook). |
| DP-1.3 | SOC 2 timing | Architect for readiness from day 1; pursue certification when a paying customer requires it |
| DP-2.1 | Event bus (NATS vs Kafka) | **Neither for v1.** Postgres LISTEN/NOTIFY or in-process channels. Add real bus only if needed |
| DP-2.2 | Agent runtime language | Go-native with direct LLM API calls. Python sidecar deferred to v1.5 only if needed |
| DP-2.3 | Identity stack | OIDC interface in code; Dex/Keycloak/Authelia for self-host, WorkOS for managed cloud |
| DP-2.5 | Core language | **Go.** Cross-platform single-binary requirement settles it. AI-assisted development negates learning ramp |
| DP-2.6 | Search backend | Three-tier: SQLite FTS5 (local) → OpenSearch (server self-host) → Elasticsearch (managed cloud) |
| DP-2.7 | SQLite as default for local | Yes. CLI local mode uses SQLite for state and FTS5 for search. Zero external deps |
| DP-2.8 | CLI-first ordering | CLI v0.1 → API stable v0.2 → web UI v1.0+ |
| DP-2.9 | **Binary distribution model** | **Two separate binaries**: `prow` (CLI, analyst-installed, no UI assets, ~15–25 MB) and `prowd` (server, engineer-deployed, includes API + daemon + embedded web UI). CLI talks to server over HTTPS. Solo/local users install `prowd` separately if they want a daemon — no auto-embedded daemon mode in the CLI. |
| DP-3.1 | PCS scope at v1 | All 4 entity types from day 1 (Event, Asset, Identity, Detection) |
| DP-3.2 | Raw payload retention | Default 1 year, configurable per tenant; raw TTL ≥ event TTL always |
| DP-3.3 | Search store | Elasticsearch/OpenSearch alone for v1; revisit dedicated analytics store only if needed |
| DP-3.4 | Multi-tenancy | Single-tenant for self-host. Hybrid for managed: shared Postgres with row-level security + index-per-tenant in ES, cell-based at scale |
| DP-4.1 | Connector distribution | All connectors in-tree for v1. Plugin architecture deferred to v2 |
| DP-4.2 | Phase 0 connector | **Wazuh** (open-source, runnable locally, exercises Event + Asset entities) |
| DP-4.3 | Phase 1 second connector | **Elastic Security** (existing expertise, accelerates framework battle-testing) |
| DP-4.4 | Connector versioning | Independent of Prow binary versions |
| DP-5.1 | Entity model | 4 entities: Event, Asset, Identity, Detection. Vulnerabilities stay as Events with `category=vuln` |
| DP-5.2 | Severity ladder | Ordinal: `info \| low \| medium \| high \| critical`. CVSS exposed in `labels` for vuln events |
| DP-5.3 | ID format | Type-prefixed ULIDs (`evt_`, `asst_`, `ident_`, `dtct_`, `inv_`, `csnt_`, `aud_`, `tnt_`, `run_`) |
| DP-5.4 | Asset merging | Simple deterministic in v1 (hostname + cloud_resource_id). Proper entity resolution as v2 |
| DP-6.1 | Agent default state | **Off by default** at install. Tenants explicitly enable |
| DP-6.2 | Local LLM at MVP | **Yes.** Ship Anthropic + OpenAI + Ollama + OpenAI-compatible together in Phase 2 |
| DP-6.3 | Consent timeout | 24 hours default, tenant-configurable per tool |
| DP-6.4 | Agent reasoning capture | Both: free-text reason for humans, structured `EvidenceRefs` for audit |
| DP-6.5 | `agent watch` mode | Hold for v1.5. MVP ships `triage` and `ask` only |

---

## Open Questions

Questions raised during planning, not yet resolved:

1. **License decision (DP-1.2).** Must be made before Phase 1 ships publicly. Recommended deep-dive: read Sentry, HashiCorp, MongoDB, Elasticsearch license-change post-mortems before deciding.
2. **Initial Phase 1 commercial connector** — CrowdStrike vs Defender for Endpoint. Both are reasonable; pick based on which has better API documentation when you start.
3. **Documentation site tooling** — Docusaurus vs VitePress vs Mintlify. Defer until Phase 1.
4. **Project name finalization** — Confirm `prow.sh` domain, `prow-sh` GitHub org, package names. Trademark check before Phase 0 publish.

---

## Part 1 — Product Foundations & Architectural Principles

### 1.1 Product summary

Prow is a **unified control plane for SecOps teams** that consolidates alerts, investigations, identities, assets, and detections from disparate security tools into a single operational layer.

- **Operating model:** Connect → Normalize → Operate → Audit
- **Trust model:** Read by default. Write with consent. Audit always.
- **Deployment model:** Open-source self-host with BYOLLM, *or* managed cloud
- **Connectors at launch:** 16+ across EDR, SIEM, vuln, IAM, cloud, ticketing, with strong emphasis on open-source security tools (Wazuh, Elastic Security, Suricata, OSSEC, TheHive+Cortex, MISP, Velociraptor, Falco, Graylog, OpenVAS) alongside commercial vendors (Defender, CrowdStrike, SentinelOne, Splunk, Sentinel, Rapid7, Tenable, Qualys, Okta, Entra ID, AWS, Azure, GCP, Jira, ServiceNow)
- **AI agent layer:** Investigates and triages on behalf of the analyst, with explicit consent gates for any write action

### 1.2 Architectural principles (non-negotiables)

1. **Read-first, write-gated.** Every connector starts read-only. Write capabilities are opt-in per tenant, per connector, per action, with a consent flow and full audit trail.
2. **Schema is sovereign.** All ingested data normalizes to the Prow Common Schema (PCS) before it's queryable. No leaky vendor abstractions.
3. **Open core, closed orchestration.** Connectors, schema, and agent runtime are open source. Multi-tenant orchestration, billing, hosted LLMs, and managed infrastructure are commercial.
4. **Audit is a first-class data type.** Every action — human or agent, read or write — produces an immutable audit event. Auditability is not bolted on; it's the spine.
5. **Tenant isolation by default.** Even self-host single-tenant runs as if isolation matters. Managed multi-tenant becomes a config flip.
6. **Connector failures are isolated.** One vendor's broken API doesn't take down triage for the other 15.
7. **The agent is a tool user, not a pipeline.** It picks tools and reasons; it doesn't have hardcoded "run X then Y then Z" flows that rot.

---

## Part 2 — System Architecture (Service Topology)

### 2.1 The two-binary model

Prow ships as **two separate Go binaries**:

| Binary | Audience | What it is | Size |
|---|---|---|---|
| `prow` | Analysts, end users | Thin CLI client. No connector code, no UI assets, no agent runtime. Knows only how to talk to a `prowd` server over HTTPS. | ~15–25 MB |
| `prowd` | Engineers, ops, deployers | Full server: HTTP API, scheduler, connector engine, notifier engine, agent runtime, audit ledger, consent state machine, embedded web UI. | ~40–60 MB |

The CLI is what an analyst downloads and installs on their laptop. The server is what an engineer or operations team deploys on a VM, container, or k8s cluster. **The CLI talks to the server over HTTPS.** The server is the source of truth.

This separation aligns the *binary boundary* with the *audience boundary* — analysts don't carry around server code they never run; engineers don't ship the entire server stack to every analyst's laptop on every upgrade.

### 2.2 Logical structure

```
prow/
├── cmd/
│   ├── prow/                # CLI binary entrypoint — analyst-facing, light
│   └── prowd/               # Server binary entrypoint — daemon, API, web UI
├── internal/
│   ├── cli/                 # CLI subsystem (Cobra commands, profiles, output formatting)
│   ├── client/              # API client used by `prow` to call `prowd`
│   ├── api/                 # HTTP API server — `prowd` only
│   ├── daemon/              # Long-running scheduler & workers — `prowd` only
│   ├── web/                 # Static web UI server — `prowd` only, embeds UI assets
│   ├── connectors/          # Connector engine + per-vendor packages — `prowd` only
│   │   ├── wazuh/
│   │   ├── elastic/
│   │   ├── crowdstrike/
│   │   └── ...
│   ├── notifiers/           # Notifier engine + per-channel packages — `prowd` only
│   │   ├── cli/
│   │   ├── slack/
│   │   ├── teams/
│   │   ├── discord/
│   │   └── email/
│   ├── pcs/                 # Schema validators (consumed by both binaries via pkg/pcs)
│   ├── store/               # Storage abstraction (SQLite | Postgres | ES | OpenSearch) — `prowd` only
│   ├── agent/               # Agent runtime + LLM provider adapters — `prowd` only
│   │   ├── llm/             # OpenAI, Anthropic, Bedrock, Ollama, OpenAI-compatible
│   │   └── tools/           # Tool definitions agent can call
│   ├── audit/               # Append-only audit ledger, hash-chained — `prowd` only
│   ├── consent/             # Write-gate consent state machine — `prowd` only
│   └── auth/                # OIDC client, local users, RBAC — `prowd` only
├── ui/                      # Next.js app — built into static assets, embedded into prowd via embed.FS
└── pkg/                     # Public packages (importable by both binaries + third parties)
    ├── pcs/                 # Public schema types
    ├── connector/           # Connector author SDK
    ├── notifier/            # Notifier author SDK
    └── client/              # Public Go SDK for talking to prowd
```

The CLI binary's import graph is intentionally narrow: `cmd/prow` → `internal/cli` → `internal/client` → `pkg/pcs` + `pkg/client`. **The CLI does not import any connector code, agent runtime, audit ledger, or storage adapter.** That's by design — those subsystems live in `prowd` and are only accessible over the API.

### 2.3 Tech stack

| Concern | Pick | Why |
|---|---|---|
| Core language | **Go** | Single static binary, native cross-platform compilation, lingua franca of self-hosted infra |
| CLI framework | Cobra + Viper | Standard for Go CLIs (kubectl, gh, docker) |
| Production state (`prowd`) | Postgres 16 | Source of truth for tenant config, users, consents, investigations, hot audit |
| Local state (`prowd` lab/eval mode) | SQLite via `modernc.org/sqlite` (pure Go, no CGO) | Zero external deps for evaluators running `prowd` locally |
| Search (lab/eval) | SQLite FTS5 | Lightweight, sufficient for <100K events/day |
| Search (server self-host) | OpenSearch | MIT-compatible Elasticsearch fork |
| Search (managed cloud) | Elasticsearch | Existing expertise, mature operational tooling |
| Object store | S3-compatible (Wasabi, Backblaze, R2, AWS S3) | Raw payloads, agent artifacts, archived audit |
| Job runner | River (Postgres-backed) for server mode; built-in queue for SQLite mode | |
| Event bus (v1) | Postgres LISTEN/NOTIFY or in-process channels | Defer NATS/Kafka until needed |
| Web UI | Next.js 15, statically exported, embedded into `prowd` via `embed.FS` | UI ships with the server, served at `/` from same origin as API. No separate UI hosting. |
| CLI ↔ server transport | HTTPS + JSON over REST (gRPC for v2 if needed) | Standard, debuggable, works through corporate proxies |
| CLI auth | OAuth device flow → token stored in OS keychain | Familiar pattern (gh, fly, gcloud) |
| Auth (server, self-host) | OIDC (Dex, Keycloak, Authelia) + local users for tiny deploys | No vendor lock for OSS users |
| Auth (server, managed) | WorkOS for SSO/SCIM | Same OIDC interface; just point at WorkOS |
| Distribution | GitHub Releases, Homebrew, apt/yum, Docker, Helm | Multi-channel for diverse audiences |

### 2.4 Three deployment modes

| Mode | What an analyst installs | What an engineer deploys | Storage | Auth |
|---|---|---|---|---|
| **Lab / Evaluation** | `prow` CLI on laptop | `prowd` on the same laptop, talking to localhost | SQLite + local FS | Local token |
| **Self-Hosted Server** | `prow` CLI on laptop, points at team's prowd URL | `prowd` on VM/container/k8s | Postgres + OpenSearch + S3-compatible | OIDC |
| **Managed Cloud** | `prow` CLI on laptop, points at managed endpoint | Prow team operates `prowd` at scale | Postgres + Elasticsearch + S3 | WorkOS |

The same `prowd` binary serves all three. Mode is config-driven. The same `prow` CLI talks to all three; profiles let one analyst switch between them.

### 2.5 The CLI ↔ server protocol

The CLI is a thin client. All it stores locally:

- Server URL(s) and OAuth tokens (in OS keychain — Keychain Access on macOS, Credential Manager on Windows, Secret Service on Linux)
- Profile config (`~/.prow/config.yaml`) — list of named server profiles
- Optional command tab-completion cache

The CLI **does not** hold connector data, alerts, asset records, agent state, or audit entries locally. Those all live in `prowd`. The CLI is a render-and-input layer over the API.

```bash
# Point the CLI at a server
prow login https://prow.acme.corp                            # interactive OAuth/OIDC flow
prow login https://prow.acme.corp --token $PROW_TOKEN        # for CI/scripts

# Multiple environments via profiles
prow login https://prow-prod.acme.corp --profile prod
prow login https://prow-staging.acme.corp --profile staging
prow --profile prod alerts
prow --profile staging connector list
```

### 2.6 Public Go SDK (`pkg/client`)

Because the CLI talks to `prowd` over HTTPS, the API is necessarily clean enough that **anyone can write a third-party tool against it.** The `pkg/client` package is the official Go SDK; it's the same code the `prow` CLI uses internally. Third parties can build dashboards, custom integrations, or alternate UIs without forking Prow.

This is a downstream benefit of the binary split: a clean API contract is forced into existence rather than discovered late.

---

## Part 3 — Data Architecture

### 3.1 Storage strategy by mode

```
Local mode:    SQLite (state + FTS5 search) + local FS (raw payloads)
Server mode:   Postgres (state) + OpenSearch (search) + S3-compatible (raw)
Managed mode:  Postgres (state) + Elasticsearch (search) + S3 (raw)
```

The storage interface in code abstracts over all three. Connectors and the agent runtime don't know which backend they're hitting.

### 3.2 Multi-tenancy isolation

For managed cloud:

- **Postgres:** single DB, `tenant_id` column on every row, **row-level security policies enforced by Postgres**, not just app code. Belt and suspenders.
- **Elasticsearch:** index-per-tenant with naming convention `prow-events-{tenant_id}-{yyyy-mm}`. Cap tenants per cluster; spin up new cluster ("cell") at 200–500 tenants.
- **S3:** bucket-per-environment, prefix-per-tenant. Signed URLs only; never direct bucket access from clients.

For self-host: single-tenant per deployment. The same `tenant_id` discipline applies but there's only one tenant.

### 3.3 Index lifecycle (Elasticsearch/OpenSearch)

- **Hot tier** (fast SSDs): events from last 30 days
- **Warm tier** (slower SSDs / fewer replicas): 30–180 days
- **Cold tier** (HDD-backed nodes, snapshots to S3): 180+ days, retrieval-on-demand
- ILM-driven, retention configurable per tenant tier

---

## Part 4 — The Connector Framework

This is the most important part of Prow's architecture. Get this right and adding the 17th, 27th, 47th connector is mechanical.

### 4.1 The connector contract

Every connector implements one Go interface:

```go
package connector

type Connector interface {
    // Metadata: identity, version, capabilities, config schema.
    Manifest() Manifest

    // Lifecycle: validate config, test connection, optional setup.
    Validate(ctx context.Context, cfg Config) error
    Test(ctx context.Context, cfg Config) (TestResult, error)

    // Reads: streams of normalized PCS entities.
    Sync(ctx context.Context, req SyncRequest, out EntityStream) error

    // Writes: optional. Only implemented if connector supports actions.
    Actions() []ActionDef
    Execute(ctx context.Context, action ActionCall) (ActionResult, error)
}
```

### 4.2 What the framework provides for free

The connector author **does not write code for**:

- HTTP transport with timeouts, TLS, proxy, retry, tracing
- Auth (BearerToken, APIKey, BasicAuth, OAuth2, AWSSigV4, mTLS, Custom)
- Rate limiting (token bucket, per-instance)
- Retries (exponential backoff with jitter)
- Pagination helpers (Cursor, Offset, LinkHeader)
- Cursor / incremental sync persistence
- Idempotency / dedup against `(tenant_id, connector, vendor_event_id)`
- PCS schema validation
- Scheduling
- Health and observability
- Encrypted credential storage
- Audit logging
- Structured logging with context

### 4.3 What the connector author writes

Manifest (declares config schema, auth type, rate limit, capabilities, sync intervals), `Test`, `Sync` (per entity type), and optionally `Actions` + `Execute` for write actions. A complete read-only connector is ~300 lines of Go.

### 4.4 The actions model

Every write action declared in the manifest:

```go
Actions: []connector.ActionDef{
    {
        Name:        "contain_host",
        DisplayName: "Contain host (network isolation)",
        Description: "Removes the host from the network. Host can still talk to EDR cloud.",
        RiskLevel:   connector.RiskHigh,
        Reversible:  true,
        ReverseAction: "uncontain_host",
        Inputs: []connector.Field{
            {Name: "asset_id", Type: "ref:asset", Required: true},
            {Name: "reason", Type: "string", Required: true},
        },
        ConsentPolicy: connector.ConsentPolicy{
            Default: connector.ConsentRequired,
        },
    },
}
```

The framework uses these declarations to render consent UIs, enforce tenant policies, generate audit records, and power the agent's tool catalog.

**The agent never invokes `Execute` directly.** It calls the consent service, which (if approved) calls the framework, which calls `Execute`.

### 4.5 Connector test harness

The framework provides record/replay HTTP transport so contributors can write connectors without live vendor environments:

```go
func TestWazuhSync(t *testing.T) {
    h := connector.NewTestHarness(t)
    h.LoadFixture("alerts_page_1.json")
    h.LoadFixture("alerts_page_2.json")

    events := h.RunSync(&Wazuh{}, pcs.EntityEvent)

    require.Len(t, events, 47)
    require.Equal(t, pcs.SeverityHigh, events[0].Severity)
    h.AssertNoSchemaViolations()
}
```

### 4.6 Distribution

For v1: all connectors in-tree, compiled into the `prow` binary. For v2: HashiCorp-style subprocess plugins (gRPC over stdio).

### 4.7 Open-source contribution flow

1. `prow connector new <vendor>` scaffolder generates stubs, test harness, fixtures dir, doc template
2. Contributor implements `Manifest`, `Test`, `Sync`
3. Contributor adds fixtures for vendor response shapes
4. `prow connector lint <vendor>` checks PCS conformance, manifest completeness
5. PR; CI runs lints + harness tests
6. Maintainer reviews PCS mapping; framework concerns already handled

---

## Part 5 — The Prow Common Schema (PCS)

### 5.1 Design principles

1. **Vendor-agnostic.** Generic concepts that map cleanly from many vendors.
2. **Lossy by design.** PCS captures the queryable 80%; full vendor payload preserved in `raw_payload_uri`.
3. **Versioned, additive.** Every entity stamped with `pcs_version`.
4. **Refs not embeds.** Events reference Assets and Identities by ID.
5. **Time is non-negotiable.** Every entity has `occurred_at` (source) and `ingested_at` (Prow).
6. **Severity is normalized.** Vendors map to `info | low | medium | high | critical`.

### 5.2 The four entities

**Event** — alerts, logins, vuln findings, audit logs, ticket updates, telemetry. Most-queried entity.

```go
type Event struct {
    PCSVersion  string
    TenantID    TenantID
    EventID     EventID            // evt_<ULID>
    IngestedAt  time.Time
    OccurredAt  time.Time
    Source      Source             // connector, vendor_event_id, raw_payload_uri
    Category    Category           // alert | login | audit | vuln | ticket | telemetry
    Severity    Severity           // info | low | medium | high | critical
    Title       string
    Summary     string
    Actors      []ActorRef         // identities + assets involved
    Indicators  []Indicator        // IPs, hashes, domains
    Tags        []string
    Labels      map[string]string
    Links       Links              // vendor URL, investigation ref
}
```

**Asset** — hosts, containers, cloud VMs, S3 buckets, SaaS apps, IoT devices. Multiple connectors merge via deterministic correlation (hostname + cloud_resource_id in v1).

**Identity** — users, service accounts, roles, API keys. Anything that can act.

**Detection** — rules, signatures, analytics. The "things that fire" rather than "things that fired."

### 5.3 ID strategy

All IDs are **ULIDs** with type prefixes:

| Type | Prefix |
|---|---|
| Tenant | `tnt_` |
| Event | `evt_` |
| Asset | `asst_` |
| Identity | `ident_` |
| Detection | `dtct_` |
| Investigation | `inv_` |
| Consent | `csnt_` |
| Audit | `aud_` |
| Agent Run | `run_` |

Time-sortable, URL-safe, 26 chars, no central allocator. Type prefixes mean log lines are immediately readable.

### 5.4 Schema evolution

- Every entity persists `pcs_version`
- Migration runner can re-shape entities at read time for known versions
- Major version bumps require explicit migration step
- Old data is never silently rewritten

---

## Part 6 — The Agent Runtime + Consent State Machine

### 6.1 Design philosophy (non-negotiable)

1. The agent is a tool user, not a workflow.
2. The agent only has access to tools the tenant has explicitly granted (default: zero tools, must opt-in per connector).
3. The agent never bypasses consent.
4. The agent's reasoning is fully observable.
5. The agent is interruptible.
6. The agent has time and budget caps (max tool calls, max tokens, max wall-clock).
7. The LLM is replaceable.
8. Local-first agent operation: with Ollama or vLLM, no data leaves the deployment.

### 6.2 The agent loop

```
Run begins with: Goal + Context + Tool catalog + Budgets

  ┌────────────────┐
  │  PLAN STEP     │  LLM produces thought, intended next tool call (or finish).
  └────────┬───────┘
           ▼
  ┌────────────────┐
  │  CONSENT GATE  │  If write: block, create consent, notify, wait.
  └────────┬───────┘
           ▼
  ┌────────────────┐
  │  TOOL CALL     │  Execute against connector framework.
  └────────┬───────┘
           ▼
  ┌────────────────┐
  │  OBSERVATION   │  Result captured + persisted to audit ledger.
  └────────┬───────┘
           ▼
   Budget check ── over budget? ── HALT (always with audit)
           │
           ▼
   Loop back to PLAN, or FINISH if agent says done
```

### 6.3 Invocation surface

```bash
prow agent triage evt_01HZ...                          # single event end-to-end
prow agent ask "Is host-prod-01 compromised?"          # free-form with tool access
prow agent watch --severity high                        # standing watch (v1.5)
```

### 6.4 Tools

Three sources, all auto-discovered:

| Source | Examples |
|---|---|
| Connector reads | `wazuh.search_alerts`, `okta.get_user_logins` |
| Connector writes | `crowdstrike.contain_host`, `okta.disable_user` |
| Built-in | `pcs.search_events`, `investigation.create` |

Three layers of tenant-configurable gating: catalog (which tools agent can see), policy (consent requirements), rate (max calls per tool per run/hour).

### 6.5 Consent state machine

```
        ┌──────────┐
        │ REQUESTED│
        └────┬─────┘
       ┌─────┴─────┐
       ▼           ▼
  ┌─────────┐ ┌──────────┐
  │APPROVED │ │  DENIED  │
  └────┬────┘ └──────────┘
       ▼
  ┌─────────┐
  │EXECUTING│
  └────┬────┘
   ┌───┴───┐
   ▼       ▼
┌──────┐ ┌──────┐
│ DONE │ │FAILED│
└──────┘ └──────┘

Plus: TIMED_OUT (auto-deny after policy timeout)
      REVOKED  (approved but withdrawn before execute)
```

Consent record captures: action ref, resolved inputs, risk level, requester (agent run or user), free-text reason, structured `EvidenceRefs`, policy applied, required approvers, timing, execution result.

### 6.6 Consent policies

| Policy | Use case |
|---|---|
| `none` | Read-only or fully trusted tools |
| `required` | Single approver from authorized set |
| `quorum-N` | N distinct approvers required |
| `auto-business-hours` | Auto-approved during business hours, required after |
| `auto-low-risk` | Action's `risk_level: low` auto-approved |
| `tenant-admin-only` | Only tenant admins can approve |

### 6.7 Notifier framework (generalized from Slack-only)

Notifiers ship in-tree like connectors. Same Manifest/Validate/Test/Send pattern:

| Notifier | Phase | Notes |
|---|---|---|
| CLI | Phase 1 | Built-in, blocking prompts |
| Web UI inbox | Phase 3 | Built-in, real-time |
| Slack | Phase 4 / v1.x | Block Kit buttons + webhook |
| Microsoft Teams | Phase 4 / v1.x | Adaptive Cards + bot framework |
| Discord | Phase 4 / v1.x | Slash commands + components |
| Email | v1.x | Magic-link approval |
| PagerDuty | v2 | High-risk, time-sensitive |
| Generic webhook | v1.x | Customer routes anywhere |

### 6.8 LLM provider interface

```go
type Provider interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    Stream(ctx context.Context, req CompletionRequest) (StreamReader, error)
    Capabilities() ProviderCapabilities
}
```

Six providers: Anthropic, OpenAI, Azure OpenAI, Bedrock, Ollama, OpenAI-compatible (covers LiteLLM, Together, Groq, Fireworks, etc.). ~600 lines of Go each.

Native tool-use (Claude, GPT-4+, Llama-3.1+) passes tools directly. Models without native tool-use: framework wraps in structured-output prompt with JSON schema validation.

### 6.9 Audit ledger

- Append-only, hash-chained (each entry contains hash of previous; tampering detectable in seconds)
- Tenant-scoped
- Externalizable (continuous export to customer S3 / SIEM via syslog)
- `prow audit verify` validates the full chain

What gets audited: connector syncs, consent transitions, connector writes, agent runs (every step), user logins/MFA/role changes, policy changes, data exports.

### 6.10 What the agent must NOT do

- No prompt-injectable data without sanitization (vendor data is template-bounded; consent gates are the real safety net)
- No tool with `eval`-shaped power
- No "skip consent" mode (auto-approve still creates a consent record)
- No silent retries on write actions
- No reading across tenants in agent context

### 6.11 Deliberate non-features (v1)

- No multi-agent / "swarm" orchestration
- No fine-tuned Prow-specific model (BYOLLM means customer's model)
- No agent-to-agent delegation
- No memory beyond the run (each run stateless across runs by default)

---

## Part 7 — Deployment & Distribution

### 7.1 Two-binary anatomy

Prow ships as **two binaries**, each tuned for its audience.

**`prow` — the CLI** (~15–25 MB)
- CLI subsystem (Cobra commands, output formatting, profiles)
- API client (`internal/client`)
- Schema types from `pkg/pcs`
- Auth helpers (OAuth device flow, OS keychain integration)

**`prowd` — the server** (~40–60 MB)
- HTTP API server
- Long-running daemon (scheduler, workers)
- Connector engine + all connector implementations
- Notifier engine + all notifier implementations
- Agent runtime + LLM provider adapters
- PCS validator + migration runner
- Storage adapters (SQLite, Postgres, OpenSearch, Elasticsearch, S3)
- Audit ledger
- Consent state machine
- Auth (OIDC client + local users + RBAC)
- Web UI assets (Next.js statically exported, embedded via `embed.FS`, served at `/`)

The `prow` CLI's import graph deliberately excludes everything in the second list. Updates to a connector require a `prowd` upgrade; analysts running the CLI never see it.

### 7.2 Distribution channels (split by audience)

| Channel | Distributes | Audience |
|---|---|---|
| GitHub Releases | Both `prow` and `prowd`, signed multi-arch binaries | Both |
| `curl \| sh` installer | `prow` CLI only | Analysts |
| Homebrew tap | `prow` CLI only | Analyst macOS users |
| Scoop / WinGet | `prow` CLI only | Analyst Windows users |
| apt / yum repos | `prowd` server primarily; `prow` CLI also packaged | Engineer Linux users |
| Docker image | `prowd` only (`ghcr.io/prow-sh/prowd:latest`, multi-arch, distroless base) | Engineers running containers |
| Helm chart | `prowd` + suggested deps | Engineers running k8s |
| Terraform module | Bootstrap for managed cloud / self-host on AWS, Azure, GCP | Infra-as-code shops |

For v1 mandatory: GitHub Releases for both, `curl | sh` for `prow`, Homebrew for `prow`, Docker image for `prowd`. apt/yum, Scoop/WinGet, Helm, Terraform are post-MVP.

The asymmetry is intentional: the CLI gets easy interactive install paths; the server gets deployment-focused paths. Engineers don't `brew install prowd` — they run containers, Helm charts, or systemd-managed binaries.

### 7.3 Configuration

**CLI config** (`~/.prow/config.yaml`) is small — list of named server profiles, default profile, output preferences. Tokens live in OS keychain, not the config file.

```yaml
# ~/.prow/config.yaml
default_profile: prod
profiles:
  prod:
    url: https://prow.acme.corp
  staging:
    url: https://prow-staging.acme.corp
output:
  format: table   # table | json | yaml
  color: auto
```

**Server config** (`/etc/prowd/config.yaml` or via env vars) is where all the operational decisions live:

```yaml
# /etc/prowd/config.yaml
server:
  bind: "0.0.0.0:7777"
  external_url: "https://prow.acme.corp"
  tls:
    cert_file: /etc/prowd/tls/cert.pem
    key_file: /etc/prowd/tls/key.pem

storage:
  postgres:
    dsn_env: PROWD_POSTGRES_DSN
  search:
    backend: opensearch     # sqlite_fts | opensearch | elasticsearch
    opensearch:
      url: https://opensearch.local:9200
      username: prow
      password_env: PROWD_OS_PASSWORD
  raw_payloads:
    backend: s3
    bucket: prow-raw
    region: us-east-1

auth:
  mode: oidc
  oidc:
    issuer: https://dex.acme.corp
    client_id: prow
    client_secret_env: PROWD_OIDC_SECRET

agent:
  enabled: false             # OFF by default per DP-6.1
  llm:
    provider: anthropic
    model: claude-sonnet-4-5
    api_key_env: ANTHROPIC_API_KEY
  budget:
    max_tool_calls: 30
    max_tokens: 200000
    max_wall_clock: 5m

notifiers:
  cli:
    enabled: true
  # slack, teams, discord, email, webhook all configured here

audit:
  retention_days: 365
  archive:
    enabled: true
    backend: s3
    bucket: prow-audit-archive
```

Hierarchical override for both: built-in defaults → `/etc/...` → `~/...` → `--config` flag → env vars → CLI flags. Secrets always env-var references.

### 7.4 First-run experience

**Lab / evaluation flow** (engineer wants to try Prow on their laptop):

```
$ prowd init --lab
Welcome to Prow (lab mode).
This will run Prow locally with SQLite and no external dependencies.

✓ Created config at ~/.prow/prowd-config.yaml
✓ Initialized SQLite database at ~/.prow/prowd.db
✓ Generated local admin token (saved to ~/.prow/prowd.token)
✓ Listening on http://localhost:7777

To connect with the CLI:
  prow login http://localhost:7777 --token $(cat ~/.prow/prowd.token)

Then try:
  prow connector add wazuh
  prow alerts
  prow doctor
```

**Server deployment flow** (engineer deploys to VM/k8s):

```
$ prowd init --mode server
? Postgres DSN: postgres://prow@db.local/prow
? Search backend: opensearch
? OpenSearch URL: https://os.local:9200
? OpenSearch username: prow
? OIDC issuer URL: https://dex.acme.corp
? External URL for this prowd: https://prow.acme.corp

✓ Validating Postgres connection...
✓ Validating OpenSearch connection...
✓ Validating OIDC discovery endpoint...
✓ Wrote /etc/prowd/config.yaml
✓ Wrote /etc/systemd/system/prowd.service
✓ Ran initial migrations

Start with:
  systemctl start prowd
  systemctl enable prowd

Then your team can install the CLI and run:
  prow login https://prow.acme.corp
```

**Analyst flow** (someone on the team wants to use Prow):

```
$ brew install prow-sh/tap/prow
$ prow login https://prow.acme.corp
Opening browser for authentication...
✓ Logged in as jonah@acme.corp (admin)
✓ Token saved to keychain

$ prow alerts
[shows alerts from the server]
```

Each audience gets the friction-shape it expects.

### 7.5 Upgrade flow

CLI and server upgrade independently — that's a key benefit of the split.

**`prow` CLI upgrade:**

```bash
prow upgrade                     # check, prompt, replace
prow upgrade --to v1.2.3
prow upgrade --check
prow upgrade --rollback
```

CLI upgrades touch only the local binary. No state migrations. Old binary kept as `prow.prev` for rollback.

**`prowd` server upgrade:**

For binary deployments:
```bash
prowd upgrade --check               # check available version
prowd upgrade --to v1.2.3 --plan    # show migration plan, no changes
prowd upgrade --to v1.2.3           # execute with maintenance window
```

For Docker/Helm: standard image tag bumps with rolling updates. Migrations run as a pre-start hook; daemon doesn't accept traffic until migrations complete.

Upgrade safety:
1. Verify signature on new binary (cosign)
2. `prowd upgrade --plan` shows pending DB + PCS migrations
3. Operator confirms
4. Maintenance: stop accepting new connector syncs and agent runs
5. Migrations run with `--apply`
6. New binary atomically replaces old; daemon hot-reloads
7. Post-upgrade `prowd doctor` confirms health
8. Resume sync schedule

**Compatibility:** CLI version N talks to server versions N-1, N, and N+1. Three-version sliding window. Older or newer combinations log a warning and may refuse certain commands.

### 7.6 Observability

`prowd` exposes:
- **Metrics:** Prometheus exposition at `/metrics`. Standard names: `prow_connector_sync_duration_seconds`, `prow_consent_resolution_seconds`, `prow_agent_run_duration_seconds`, `prow_agent_llm_tokens_total`, `prow_api_request_duration_seconds`, etc.
- **Traces:** OpenTelemetry. Customer points at any OTLP collector (Tempo, Jaeger, Honeycomb, Datadog).
- **Logs:** Structured JSON with `tenant_id`, `trace_id`, `subsystem`, `event` fields.
- **Self-monitoring:** `prowd doctor` for human-readable health checks across all subsystems.

The CLI exposes minimal observability — it's a client. `prow doctor` runs against the connected server and reports its health, plus checks local concerns (token validity, network reachability).

### 7.7 Backup and disaster recovery

CLI has nothing meaningful to back up — it's a client. Re-running `prow login` restores it.

Server (`prowd`) backup:
- **Lab mode:** `prowd backup create` snapshots SQLite + raw FS to a directory or S3.
- **Server mode:** pg_dump + OpenSearch snapshots + S3 raw payload versioning. Documentation provides reference scripts; the framework doesn't run them — that's the customer's ops territory.
- **Managed mode:** continuous PITR Postgres, ES snapshots every 6h cross-region, raw payloads in CRR-replicated S3. RPO 15min, RTO 1hr.

### 7.8 Security hardening

- TLS by default on `prowd` (refuses non-TLS in server mode without `--insecure-allow-http`)
- mTLS option for CLI-to-server in zero-trust environments
- CLI tokens stored in OS keychain only — never written to disk in plain form
- Server-side secrets at rest encrypted with envelope encryption (KMS, OS keychain, or env-var key)
- Connector credentials never logged, redacted from audit payloads
- Cosign-signed releases for both binaries, CycloneDX SBOMs published
- govulncheck in CI, dependency review on every PR
- CSP headers on embedded web UI; no eval, no inline scripts, no third-party origins
- The web UI is served from the same origin as the API — no CORS surface, no third-party-CDN dependency

---

## Part 8 — Phasing & Roadmap

### 8.1 Phase 0 — Walking Skeleton (~2-3 weeks)

End-to-end product slice. Project scaffold with **two binary builds** (`prow` CLI + `prowd` server) under one Go module. `prowd init --lab`, `prow login`, `prow doctor`, SQLite-only storage in lab mode, PCS v0.1 Events only, framework MVP, Wazuh connector, basic CLI, audit ledger, GitHub Releases publishing both binaries.

**Exit:** Fresh-machine install (`prowd` lab mode + `prow` CLI) → Wazuh connector → see alerts in <5min. CI produces signed binaries for both `prow` and `prowd`.

**Demo:** 2-min screencast posted publicly. Show `prowd init --lab` on one terminal, `prow login` and `prow alerts` on another.

### 8.2 Phase 1 — MVP (~10-14 weeks)

Real, useful product. Add Elastic Security + one commercial connector (CrowdStrike or Defender). PCS v0.2 with Asset and Identity. Asset merging. Investigations CLI. **`prowd` server mode** (Postgres + OpenSearch + OIDC) — engineer-deployable on a VM. Notifier framework + CLI notifier. Independent `prow upgrade` and `prowd upgrade` flows with three-version compatibility. Documentation site. Public Go SDK at `pkg/client`.

**Exit:** Three working connectors. `prowd` server mode boots clean on a fresh Ubuntu 24.04. Connector contributor docs published. Upgrade tested across both binaries. CLI/server compatibility matrix documented.

**Demo:** "Self-host Prow in 10 minutes" (deploys `prowd`) + "Write a connector in an afternoon" (contributes to `prowd`).

### 8.3 Phase 2 — The Agent (~8-10 weeks)

LLM adapters (Anthropic + OpenAI + Ollama + OpenAI-compatible). Agent runtime. Tool catalog auto-derived. Tool gating per tenant. `prow agent triage` and `prow agent ask`. Consent state machine. First write-capable connector (CrowdStrike `contain_host`). Audit replay.

**Excluded:** `agent watch`, multi-agent, cross-run memory, Slack/Teams/Discord (Phase 4).

**Exit:** End-to-end demo: alert → triage → consent → write → investigation. Replayable from audit. Llama 3.1 70B via Ollama completes triage end-to-end.

**Demo:** "Watch Prow triage a real alert" — 3 min.

### 8.4 Phase 3 — Web UI (~8-12 weeks)

Next.js statically exported, **embedded into `prowd` only** (not the CLI). Triage inbox, investigations, consent inbox, audit viewer with chain verification, connector management, agent run viewer. Read-only initially; mutations through same API as CLI. Served at the root path of the `prowd` server, same origin as the API — no CORS, no third-party CDN dependency.

**Exit:** Feature parity with CLI for read paths. UI mutations through identical API. `prow` CLI binary size unchanged (UI is purely a `prowd` concern). UI deployment is automatic — engineers who upgrade `prowd` get the new UI for free.

**Demo:** "Prow's web UI in 90 seconds."

### 8.5 Phase 4 — Ecosystem & Polish (rolling)

Notifiers: Slack, Teams, Discord, email, generic webhook. Connector expansion: SentinelOne, Splunk, Sentinel, Tenable, Qualys, Okta, Entra ID, AWS, Azure, GCP, Jira, ServiceNow, plus open-source TheHive, MISP, Velociraptor, Falco, Suricata, Graylog, OpenVAS. `prow agent watch`. Performance benchmarking. SOC 2 Type II readiness.

### 8.6 Phase 5 — Managed Cloud (when traction warrants)

Multi-tenant deployment. Cell-based architecture (200-500 tenants/cell). WorkOS SSO/SCIM. Stripe billing. Customer admin console. Onboarding flow. Status page, SLAs. Continuous audit export to customer S3.

### 8.7 Cross-cutting workstreams

- Documentation (Docusaurus or VitePress, in-repo, updated with every behavior-changing PR)
- Performance benchmarks in CI
- Quarterly external security review starting Phase 2
- Bug bounty starting Phase 3 launch
- Community Discord/Slack starting Phase 1

### 8.8 Realistic timeline

| Phase | Effort | Cumulative |
|---|---|---|
| Phase 0 | 2-3 weeks | ~3 weeks |
| Phase 1 | 10-14 weeks | ~4 months |
| Phase 2 | 8-10 weeks | ~6.5 months |
| Phase 3 | 8-12 weeks | ~9 months |
| Phase 4 | rolling | year 1+ |
| Phase 5 | rolling | when traction warrants |

~9 months solo-with-AI to CLI-complete + agent + web UI. Aggressive but plausible if Prow is primary focus.

---

## Part 9 — Risks I'm Tracking

Ranked by probability × impact. Read this section first before each phase boundary.

### 9.1 Connector tax (HIGH × HIGH)

80% of work, 100% of customer value. 16 commercial connectors at 3-4 weeks each = 12 months on connectors alone.

**Mitigations:** Frameworks compounds. Open-source vendors first (no vendor account needed). Prioritize commercial connectors ruthlessly. AI-assisted authoring. Sponsorship/partnership for commercial connectors.

**Watch for:** Burning >4 weeks on one vendor. Ship without it.

### 9.2 Agent reliability (HIGH × HIGH)

LLMs hallucinate. Agents loop. SecOps people will hunt for failure modes.

**Mitigations:** Budget caps as architecture. Schema-validated tool I/O. Conservative consent defaults. Adversarial testing in CI. Replayable audit traces. Don't oversell — "investigates and triages on behalf of analyst," not "autonomous SOC."

**Watch for:** Demo-day temptation to skip consent "for time."

### 9.3 Storage scaling cliff (MEDIUM × HIGH)

SQLite is plenty until it isn't. Customer plugs in high-volume connector, SQLite collapses, blames Prow.

**Mitigations:** Document tier ladder explicitly (<100K events/day = SQLite). `prow doctor` warns at thresholds. SQLite → Postgres + OpenSearch migration tool in Phase 1. Don't oversell SQLite.

### 9.4 Open source vs. commercial confusion (MEDIUM × MEDIUM)

License decision deferred. Architecture is neutral; positioning isn't. Decision is harder later.

**Mitigations:** Decide before Phase 1 ships publicly. Read Sentry/HashiCorp/Mongo/Elastic license-change post-mortems. Default recommendation: BSL with 4-year Apache 2.0 conversion clause.

**Watch for:** Avoiding the decision. It will be harder later.

### 9.5 Multi-tenancy bugs cross-tenant data (LOW × EXISTENTIAL)

Single bug = product extinction. Security buyers don't recover.

**Mitigations:** Postgres RLS as second line. Index-per-tenant ES as third line. API gateway forbids cross-tenant by construction. Pen test focused on tenant boundary before managed cloud GA. Cell-per-customer option for high-paranoia buyers.

**Watch for:** First "for debugging" cross-tenant query. Harden boundary in code, not later.

### 9.6 Solo / small team burnout (HIGH × HIGH)

9 months focused work. SecOps is grinding through Phase 4. Solo founders burn out.

**Mitigations:** Phase 0 ships fast — 3 weeks to a real artifact. Get help if any way possible. Framework for contributor connectors. Public roadmap as forcing function.

**Watch for:** Skipping Phase 0 screencast/blog in favor of "more code." Public artifacts compound.

### 9.7 Differentiation erosion (MEDIUM × MEDIUM)

"Unified SecOps + AI agent" is not unique. Splunk + Microsoft moving same direction.

**Mitigations:** Open-source-first is the moat. BYOLLM is the second moat. Connector contribution model is the third. "Audit always" is positioning moat.

**Watch for:** Drift toward feature parity. Right move is feature *non-parity* in chosen dimensions.

### 9.8 SOC 2 / compliance creep (MEDIUM × LOW-MEDIUM)

Each framework is 6-month, $50K+ engagement. They stack.

**Mitigations:** Architecture-as-readiness. SOC 2 Type II first (most common). Don't chase proactively — wait for paying customer.

**Watch for:** Shipping features that make compliance harder.

### 9.9 Vendor relationship risk (LOW × MEDIUM)

Major vendor decides Prow is "unauthorized." C&D. PR mess.

**Mitigations:** Documented public APIs only. Apply for partner programs. No scraping. If a vendor objects, comply, then publish the story.

**Watch for:** ToS changes specifically prohibiting "control plane" use.

### 9.10 Agent scope creep (HIGH × MEDIUM)

Agents are fun. Each "agent does X" feature extends Phase 2 by weeks. Phase 3 slips six months.

**Mitigations:** Phase 2 scope locked. Additions are "v1.5 candidate" not "v1 must-have." Force-rank agent improvements vs. connector additions — 17th connector usually wins.

**Watch for:** Yourself, having fun. The agent is the most fun part of the codebase.

---

## Appendix — Files & Conventions

- **Repo:** `github.com/prow-sh/prow` (org and naming pending DP-1.2)
- **Go module:** `github.com/prow-sh/prow`
- **Public packages:** `pkg/pcs`, `pkg/connector`, `pkg/notifier`, `pkg/client`
- **Internal packages:** `internal/*`
- **Docs site:** `docs/` directory, built with Docusaurus or VitePress
- **License file:** TBD per DP-1.2
- **Code of Conduct:** Contributor Covenant 2.1
- **Security policy:** `SECURITY.md` with responsible disclosure email and PGP key

---

*End of document. Living doc — update as decisions evolve.*
