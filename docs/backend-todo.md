# agent-observer: backend TODO

Scope: backend only. The portal UI waits until the backend below is done.

Last updated: 2026-10-15

## Done

- [x] Reverse proxy that forwards A2A traffic and rewrites agent cards
- [x] Call tracking: discovery vs call kinds, outcomes (success / failed / processing), error classification
- [x] Grouping: `X-Trace-Id` when the client sends it, idle-gap inference per client address otherwise
- [x] Memory cleanup for the tracker and the group assigner
- [x] `GET /api/groups` and `GET /api/groups/{id}` (tested, including the failure path and 404)

## Decisions already made

| Decision | Choice |
|---|---|
| Project identity | Own ID (public) and own key (secret). Never the group ID. |
| How the proxy learns the project | `X-Observer-Key` header (path-prefix fallback only if the Python A2A client cannot send headers) |
| Where routes come from | Per-project agent registry (name to upstream URL), registered through the API. No hardcoded `routes` map. |
| Storage | Postgres for projects and agents. Groups and calls stay in memory for now, behind an interface so they can move later. |
| Keys at rest | Hashed. The plain key is shown once, in the create response. |
| No key on a request | Attributed to an auto-created `default` project |
| Wrong key | 401 with a clear message |

## Phase 1: restructure the folder

Goal: same behaviour, new layout. Nothing new gets built in this phase.

- [ ] Pick the module path in `go.mod` (for example `github.com/<you>/agent-observer`); internal imports depend on it
- [ ] Create the layout:
  ```
  cmd/proxy/main.go          wiring only: flags, store, routes, ListenAndServe
  internal/outcome/          call_outcome.go
  internal/grouping/         client_calls.go (GroupAssigner)
  internal/tracker/          Tracker, records, views, cleanup (memory_growth.go, output.go)
  internal/proxy/            handler, ReverseProxy setup, card rewrite
  internal/api/              getGroups, getGroup, JSON helpers
  ```
- [ ] Export what other packages need (capitalised names); keep the rest private
- [ ] Remove the temporary `assigner clients=%d` debug log
- [ ] Re-run the existing curl tests: list, detail, 404, failed discovery. Output must be identical.

## Phase 2: Docker and Postgres

- [ ] `Dockerfile`: multi-stage, `CGO_ENABLED=0`, small final image, non-root user
- [ ] `docker-compose.yml` with `proxy` and `postgres`
  - [ ] Postgres healthcheck, and `depends_on` with `condition: service_healthy`
  - [ ] Named volume for Postgres data
  - [ ] Config through env vars (`DATABASE_URL`, `PORT`), with a `.env.example` committed and `.env` ignored
- [ ] Gotcha to handle: inside the container, `localhost:8001` is the container, not your Mac. Registered upstreams for host-run agents use `http://host.docker.internal:8001`. Document this in the README.
- [ ] Database layer: pick the driver (pgx) and a migration tool (goose or golang-migrate, or embedded `.sql` files)
- [ ] First migration:
  ```
  projects(id text pk, name text, key_hash text unique, created_at timestamptz)
  project_agents(project_id text fk, name text, upstream_url text, primary key (project_id, name))
  ```
- [ ] Proxy connects at startup, runs migrations, fails fast with a clear message if the database is unreachable
- [ ] Verify: `docker compose up`, the proxy starts, the orchestrator still works through port 8080

## Phase 3: projects endpoint and agent registry

The registry is part of the project record, so these are built together.

- [ ] `ProjectStore` interface (create, list, get by ID, find by key hash) with a Postgres implementation
- [ ] Key and ID generation with `crypto/rand`; store only the key hash
- [ ] `POST /api/projects` `{name}` returns `201` with `id`, `name`, `key` (the only time the key appears)
- [ ] `GET /api/projects` and `GET /api/projects/{id}` (never include the key or hash); agents list is empty by default
- [ ] Agent registry endpoints
  - [ ] `POST /api/projects/{id}/agents` `{name, url}`
  - [ ] `GET /api/projects/{id}/agents`
  - [ ] `DELETE /api/projects/{id}/agents/{name}`
- [ ] Validation
  - [ ] Agent name: lowercase letters, digits, dashes; reject reserved words (`api`, `debug`)
  - [ ] Upstream: parses as http/https with a host (private and metadata address blocking before any hosted deployment)
  - [ ] Name unique within a project
- [ ] Seed the `default` project at startup (flags or config for its agents, so the current news/financial test setup keeps working)
- [ ] Test with curl: create, list, add an agent, remove it, bad inputs

## Phase 4: handler uses the registry

- [ ] Read `X-Observer-Key`: missing means `default`; unknown means 401
- [ ] Cache key-to-project lookups in memory (invalidate on writes) so each proxied request does not hit Postgres
- [ ] First path segment is the agent name, looked up in the project's agents; unknown agent gives a 404 that names it
- [ ] Build the ReverseProxy for that upstream (share one transport)
- [ ] Derive `publicBase` from `r.Host` and scheme, with a flag override for use behind TLS termination
- [ ] Delete the hardcoded `routes` map and `publicBase` constant
- [ ] Verify: the orchestrator works with the key header and without it

## Phase 5: scope tracker, groups and calls by project

Done after Phase 4, as its own step. The handler must already resolve a project, and doing both at once makes failures hard to place.

- [ ] `GroupRecord` gets `ProjectID`
- [ ] Tracker key becomes project plus group ID (a client-supplied trace ID must not collide across projects)
- [ ] `GroupAssigner` key becomes project plus client address
- [ ] Cleanup keeps working across projects
- [ ] New routes: `GET /api/projects/{id}/groups` and `GET /api/projects/{id}/groups/{gid}`
- [ ] Keep `/api/groups` as an "all projects" alias until the UI switches
- [ ] Project summary counts on `GET /api/projects` (groups, running, failed, last activity)
- [ ] Verify: two projects sending the same `X-Trace-Id` end up in separate groups, and neither can read the other's

## Phase 6: hardening and config

- [ ] Flags or env for `-idle-gap`, `-retention`, `-cleanup-every`, port
- [ ] JSON error body for unknown group and unknown project
- [ ] CORS wrapper for a UI on another port
- [ ] Auth on the management API (`POST /api/projects` is open for now; fine on localhost, not hosted)
- [ ] Optional `.UTC()` on timestamps
- [ ] Tests: `GroupAssigner`, `outcomeFromRPCBody`, `Snapshot`, project store (against the compose Postgres)
- [ ] Verify the real JSON-RPC method name and task-state spelling in live POST bodies

## Phase 7: agent-side events (after the backend above)

- [ ] Proxy forwards `X-Observer-Group` and `X-Observer-Call` to agents
- [ ] `POST /api/events`, authenticated by the project key; events stored on the matching call and nested in the group detail
- [ ] Python package: `span` helper, fire-and-forget, errors swallowed, no raw prompts or arguments by default
- [ ] Instrument the Tavily and Finnhub MCP calls, then LLM calls (tokens from `usage`)
- [ ] Orchestrator reporter: trace ID per question and "planned agents"
- [ ] npm package

## Later

- [ ] Portal UI (project switcher first, then groups, then call detail)
- [ ] Streaming support (`peekBody` and duration logic assume non-streaming)
- [ ] Move groups and calls to Postgres for history, if wanted
- [ ] Hosted deployment: TLS, real auth, private-address blocking
