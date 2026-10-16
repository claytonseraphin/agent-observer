# agent-observer: backend TODO

Scope: backend only. The portal UI waits until the backend below is done.

Last updated: 2026-10-15 (Phase 1 code complete)

## Done

- [x] Reverse proxy that forwards A2A traffic and rewrites agent cards
- [x] Call tracking: discovery vs call kinds, outcomes (success / failed / processing), error classification
- [x] Grouping: `X-Trace-Id` when the client sends it, idle-gap inference per client address otherwise
- [x] Memory cleanup for the tracker and the group assigner
- [x] `GET /api/groups` and `GET /api/groups/{id}` (tested, including the failure path and 404)
- [x] Phase 1 code restructure into `internal/` packages (see Phase 1 for the remaining housekeeping)

## Decisions already made

| Decision | Choice |
|---|---|
| Project identity | Own ID (public) and own key (secret). Never the group ID. |
| How the proxy learns the project | `X-Observer-Key` header (path-prefix fallback only if the Python A2A client cannot send headers) |
| Where routes come from | Per-project agent registry (name to upstream URL), registered through the API. No hardcoded `routes` map. |
| Storage | Postgres for projects and agents. Groups and calls stay in memory for now, behind an interface so they can move later. |
| Keys at rest | Hashed. The plain key is shown once, in the create response. |
| Postgres driver | `pgx` |
| Migrations | `goose`, with the `.sql` files embedded in the binary |
| Compose layout | Base `compose.yaml` plus per-environment overrides (dev now; staging and prod in Phase 8 once the deployment target is chosen) |
| No key on a request | Attributed to an auto-created `default` project |
| Wrong key | 401 with a clear message |

## Phase 1: restructure the folder (code done, housekeeping left)

Goal: same behaviour, new layout. Nothing new gets built in this phase.

- [x] Module path: `github.com/claytonseraphin/agent-observer`
- [x] Layout (as built):
  ```
  cmd/proxy/main.go          wiring only: creates tracker + assigner, registers routes, ListenAndServe
  cmd/proxy/cleanup.go       startCleanup + the cleaner interface (needs both tracker and assigner)
  cmd/proxy/helper.go        mustParse (temporary, goes with the routes map in Phase 4)
  internal/outcome/          call_outcome.go
  internal/grouping/         client_calls.go (GroupAssigner + its Cleanup)
  internal/tracker/          tracker.go, output.go, memory_growth.go (Tracker + its Cleanup)
  internal/proxy/            proxy.go (Proxy, New, ServeHTTP, clientAddress), card.go (rewriteCard)
  internal/api/              routes.go (Register), api.go (handlers on a struct holding the tracker)
  ```
- [x] Globals replaced by injected values (`proxy.New(tr, asg, routes, publicBase)`, `api.Register(mux, tr)`)
- [x] Re-run curl tests: success, failed discovery, 404, list, cleanup. Output identical.
- [ ] Remove the temporary `assigner clients=%d` debug log
- [ ] Restore production cleanup values: `startCleanup(time.Minute, time.Hour, tr, asg)`
- [ ] Move the loose `.md` / `.pdf` files into `docs/`
- [ ] Confirm `.env` is listed in `.gitignore`
- [ ] Always run from the repo root: `go run ./cmd/proxy`

## Phase 2: Docker and Postgres

Order: the Docker and compose items are done; do Phase 2b (CI) next, then come back for the database-layer items below.

- [x] `Dockerfile`: multi-stage, `CGO_ENABLED=0`, distroless `static:nonroot` final image, plus `.dockerignore` (keeps `.env` and `.git` out of the build context). Verified: the container runs and records the orchestrator's calls.
- [x] Temporary env config so the container can reach host agents: `PORT`, `NEWS_URL`, `FINANCIAL_URL` via `envOr` (replaced by the registry in Phase 4)
- [x] Agent card rewrite now sets the card's URL fields (`url`, `supportedInterfaces[].url`, `additionalInterfaces[].url`) instead of string-replacing the upstream URL. Needed because an agent advertises its own URL, which differs from the upstream the proxy dials (for example `localhost:8001` vs `host.docker.internal:8001`); without this the orchestrator bypassed the proxy.
- [x] Layered compose files: one base plus a small override per environment (no full copies, so settings cannot drift)
  - [x] `compose.yaml` (shared): `proxy` and `postgres`, Postgres healthcheck with `depends_on` `condition: service_healthy`, named volume for Postgres data, env var names
  - [x] `compose.dev.yaml`: proxy built from source, Postgres port published to the host for inspection, upstreams via `host.docker.internal`, verbose logging
  - [x] Run dev with `docker compose -f compose.yaml -f compose.dev.yaml up`, or set `COMPOSE_FILE=compose.yaml:compose.dev.yaml` in the local `.env` so plain `docker compose up` works
  - [ ] All environment-specific values come from env vars (`DATABASE_URL`, `PORT`, log level), nothing baked into the image, so the same image runs everywhere
  - [ ] Commit `.env.example`; real `.env*` files stay git-ignored
  - [ ] `compose.staging.yaml` and `compose.prod.yaml` are deferred to Phase 8, because they depend on the deployment target
- [x] `Makefile` with dev targets: `help`, `dev`, `down`, `reset`, `logs`, `ps`, `psql`, `check` (gofmt + vet + build), `test`, `run`. Staging and prod targets come with their compose files in Phase 8.
- [ ] Gotcha to handle: inside the container, `localhost:8001` is the container, not your Mac. Registered upstreams for host-run agents use `http://host.docker.internal:8001`. Document this in the README.
- [ ] Database layer: `pgx` as the driver, `goose` for migrations with the `.sql` files embedded (`embed`) and run at startup. New concept to learn first: `context.Context`.
- [ ] First migration:
  ```
  projects(id text pk, name text, key_hash text unique, created_at timestamptz)
  project_agents(project_id text fk, name text, upstream_url text, primary key (project_id, name))
  ```
- [ ] Proxy connects at startup, runs migrations, fails fast with a clear message if the database is unreachable
- [ ] Verify: `docker compose up`, the proxy starts, the orchestrator still works through port 8080

## Phase 2b: CI with GitHub Actions

Goes right after Phase 2, so every later phase is built with checks running. Needs only GitHub; no deployment target yet.

- [ ] Run `gofmt -w .` once and commit it on its own (the repo has mixed indentation from a plain editor); after that `make check` keeps it clean
- [ ] `.github/workflows/ci.yml`, triggered on push and on pull requests (runs the same checks as `make check`, plus tests)
- [ ] Steps: checkout, `actions/setup-go` with `go-version-file: go.mod`, then
  - [ ] `gofmt -l .` fails the job if it lists anything
  - [ ] `go build ./...`
  - [ ] `go vet ./...`
  - [ ] `go test -race ./...`
- [ ] Write the first tests so CI has something to run (pure functions, no database): `GroupAssigner`, `outcomeFromRPCBody`, `groupIsOld`, `Snapshot`
- [ ] Docker build job: build the image without pushing, to catch a broken Dockerfile
- [ ] Postgres service container in the workflow, for the store tests (once Phase 3 adds them)
- [ ] `govulncheck ./...` for known vulnerabilities in dependencies
- [ ] Dependabot for Go modules and GitHub Actions versions
- [ ] Branch protection on `main`: require the CI job to pass before merging

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
- [ ] Card rewrite points every interface at the same proxy URL, which is right for JSON-RPC over HTTP only; revisit if non-HTTP transports (gRPC) need support
- [ ] Consider a startup warning if an agent's card does not contain a URL field the rewrite recognises (the proxy would be bypassed silently)
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

## Phase 8: CD and releases (after the backend is stable)

Depends on one open decision: where the hosted portal and proxy run.

- [ ] **Decide the deployment target** (a VPS running the compose file, Fly.io, Cloud Run, or similar). The rest of this phase follows from it.
- [ ] `GET /api/health` on the proxy (liveness for the platform and for the Compose healthcheck)
- [ ] Release workflow on tags `v*`: build the image and push it to GitHub Container Registry, tagged with the version and `latest`
- [ ] Secrets (database URL, registry tokens, deploy keys) in GitHub Actions secrets or the platform's secret store, never in the repo
- [ ] Deploy job: pull the new image, run migrations, restart, wait for `/api/health`
- [ ] Rollback: keep the previous image tags and document how to redeploy one
- [ ] Optional: publish proxy binaries (GoReleaser) for people who want to run it without Docker, since the proxy is meant to be open source
- [ ] If the target is compose-based: `compose.staging.yaml` and `compose.prod.yaml` as overrides on `compose.yaml`
  - [ ] Proxy image is a pinned tag from GitHub Container Registry (no build from source)
  - [ ] Postgres port not published; only the proxy can reach it
  - [ ] `restart: unless-stopped`, resource limits, quieter logging
  - [ ] Secrets from the secret store or untracked env files, never committed
  - [ ] Prod only: a TLS reverse proxy (for example Caddy) in front of the proxy
- [ ] If the target is a platform (Fly.io, Cloud Run, ...): there is no prod compose file; the same per-environment settings go into the platform's config instead
- [ ] Staging environment, if it turns out to be needed before production

## Later

- [ ] Portal UI (project switcher first, then groups, then call detail)
- [ ] Streaming support (`peekBody` and duration logic assume non-streaming)
- [ ] Move groups and calls to Postgres for history, if wanted
- [ ] Hosted deployment hardening: TLS, real auth, private-address blocking (the pipeline itself is Phase 8)
