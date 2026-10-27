# agent-observer: backend TODO

Scope: backend only. The portal UI waits until the backend below is done.

Last updated: 2026-10-26 (CI design on staging/production, Go 1.26, Phase 3 schema written, database layer is next)

Companion doc: `claude/projects-api-schema.md` (the full Phase 3 endpoint design).

## Done

- [x] Reverse proxy that forwards A2A traffic and rewrites agent cards
- [x] Call tracking: discovery vs call kinds, outcomes (success / failed / processing), error classification
- [x] Grouping: `X-Trace-Id` when the client sends it, idle-gap inference per client address otherwise
- [x] Memory cleanup for the tracker and the group assigner
- [x] `GET /api/groups` and `GET /api/groups/{id}` (tested, including the failure path and 404)
- [x] Phase 1 code restructure into `internal/` packages (see Phase 1 for the remaining housekeeping)
- [x] Docker image, layered compose files and Makefile (Phase 2)
- [x] GitHub Actions workflow `ci.yml` is in place and running (see Phase 2b)

## Decisions already made

| Decision | Choice |
|---|---|
| Project identity | Own ID (public) and own key (secret). Never the group ID. |
| How the proxy learns the project | `X-Observer-Key` header (path-prefix fallback only if the Python A2A client cannot send headers) |
| Where routes come from | Per-project agent registry (name to upstream URL), registered through the API. No hardcoded `routes` map. |
| Storage | Postgres for projects and agents. Groups and calls stay in memory for now, behind an interface so they can move later. |
| Keys at rest | Hashed with SHA-256 (the key is 256 random bits, so bcrypt would only slow every proxied request). The plain key is shown once, in the create response. |
| Postgres driver | `pgx` |
| Migrations | `goose`, with the `.sql` files embedded in the binary |
| Compose layout | Base `compose.yaml` plus per-environment overrides (dev now; staging and prod in Phase 8 once the deployment target is chosen) |
| No key on a request | Attributed to an auto-created `default` project |
| Wrong key | 401 with a clear message |
| Projects API shape | See `claude/projects-api-schema.md`: IDs `prj_...`, keys `obs_...`, JSON error body, optional agents at create |
| Go version | 1.26. The remaining standard-library vulnerability fixes exist only in 1.26.x (1.25.14 is the last 1.25 patch). CI takes the latest patch. |
| CI branches | `staging` and `production` (no `main`). Checks on every push; Docker build and govulncheck only on those two. |
| Container registry | ECR, pushed from GitHub Actions with OIDC (no AWS keys stored in GitHub) |
| Compute | Leaning ECS Fargate. Kubernetes (kind locally, a time-boxed EKS session) is a learning track only. |

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
- [ ] Confirm `.env` and `.DS_Store` are listed in `.gitignore`
- [ ] Always run from the repo root: `go run ./cmd/proxy`

## Phase 2: Docker and Postgres

Order: the Docker and compose items are done and Phase 2b (CI) is mostly done. **The database layer below is the current step.**

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

### Database layer (current step)

Do these in order. Nothing here changes what the proxy does; it only connects, creates the tables, and fails fast if the database is missing.

- [ ] Learn `context.Context` first (every `pgx` call takes one). Rules of thumb: `ctx` is always the first parameter; `context.Background()` at the top (`main`, tests); `r.Context()` inside HTTP handlers; `context.WithTimeout` plus `defer cancel()` for anything that must not hang.
- [ ] `go get github.com/jackc/pgx/v5`
- [ ] New package `internal/db/`: a connect function that takes a `ctx` and the database URL, opens a `pgxpool` pool, and pings it. Startup uses a 5-second timeout so an unreachable database is a clear error, not a hang. Error messages must never print the password from the URL.
- [ ] `DATABASE_URL` is read in `main` and is **required** (exit with a clear message if empty). Two values to remember:
  - inside compose the host is `postgres` (already in `compose.yaml`)
  - for `make run` / `go run` on your Mac, use `localhost:5432` (the dev override publishes it on 127.0.0.1), for example `postgres://observer:<password>@localhost:5432/observer`
- [ ] `go get github.com/pressly/goose/v3`. `.sql` files live in `internal/db/migrations/`, embedded with `embed`, and run at startup before the server listens. goose works on `database/sql`; pgx's `stdlib` package can give it a handle from the pool.
- [ ] First migration `0001_projects.sql` with `-- +goose Up` and `-- +goose Down` sections (columns below)
- [ ] Startup order in `main`: read config, connect (fail fast), migrate, start cleanup, listen. Close the pool on exit.
- [ ] Optional, and a good use of context: graceful shutdown with `signal.NotifyContext` and `http.Server.Shutdown` (ECS sends SIGTERM when it stops a task)
- [ ] Verify
  - [ ] `make dev`, then `make psql` and `\dt` shows `projects`, `project_agents` and `goose_db_version`
  - [ ] Restart the proxy: migrations do not run again
  - [ ] `docker compose stop postgres`, then start the proxy: it exits with a clear error and a non-zero status
  - [ ] The orchestrator still works through port 8080
- [ ] Commit (never `.env`)

First migration, as designed in `claude/projects-api-schema.md` section 3:

```
projects(
  id          text primary key,          -- 'prj_...' or 'default'
  name        text not null,
  key_hash    text unique,               -- NULL only for 'default'
  key_hint    text,                      -- last 4 chars of the key
  created_at  timestamptz not null default now()
)                                        -- plus a unique index on lower(name)

project_agents(
  project_id    text not null references projects(id) on delete cascade,
  name          text not null,
  upstream_url  text not null,
  created_at    timestamptz not null default now(),
  primary key (project_id, name)
)
```

## Phase 2b: CI with GitHub Actions

Mostly done. One workflow, `.github/workflows/ci.yml`, triggered on a push to any branch (tags excluded). A merge into `staging` or `production` is a push to that branch, so the checks run again there, followed by the Docker build and govulncheck, which run only on those two branches and only after `check` passes. The required check names are "Format, vet, build, test" and "Docker image builds".

- [x] `gofmt -w .` committed (the `check` job passes: the vulncheck job, which needs it, ran)
- [x] `ci.yml`: checkout, `actions/setup-go`, then `make check` (gofmt, vet, build) and `make test` (`go test -race ./...`)
- [ ] Docker image build job (`staging` and `production` pushes only): written; confirm it is green
- [ ] `govulncheck` job (`staging` and `production` pushes only): runs, but fails until the Go 1.26 move below
- [ ] **Move to Go 1.26** (the remaining findings are fixed only in 1.26.x):
  - [ ] `go mod edit -go=1.26.9`, then `make check && make test`
  - [ ] Both `setup-go` steps in `ci.yml`: `go-version: '1.26'` and `check-latest: true` (no `go-version-file` alongside; `go-version` wins and `setup-go` warns)
  - [ ] `Dockerfile` builder stage: `golang:1.26-alpine`
- [ ] Dependabot (`.github/dependabot.yml`): weekly for `gomod`, `github-actions` and `docker`
- [ ] Write the first tests so CI has something to run (pure functions, no database): a table-driven test for `groupIsOld` (like `pytest.mark.parametrize`), then `GroupAssigner`, `outcomeFromRPCBody`, `Snapshot`
- [ ] Branch protection on **`staging` and `production`** (not `main`): pull request required, check "Format, vet, build, test" required; on `production` also "Docker image builds" and branch up to date; vulncheck stays optional; block force pushes and deletions. Private repos need a paid GitHub plan for this to be enforced.
- [ ] Optional: add a `pull_request` trigger limited to `staging` and `production`, so the Docker build can block a merge (as it stands, it runs after the merge)
- [ ] Postgres service container in the workflow, for the store tests (once Phase 3 adds them)
- [ ] When CD arrives: stop cancelling in-progress runs on `production` (`cancel-in-progress` is on for CI only)

## Phase 3: projects endpoint and agent registry

Full design: `claude/projects-api-schema.md` (data, request and response shapes, validation, errors, curl checklist). The registry is part of the project record, so these are built together. **Starts after the database layer above.**

- [ ] Pure functions first, with unit tests: agent-name check, upstream URL check, project and key generation (`crypto/rand`), key hashing (SHA-256)
- [ ] `ProjectStore` interface (create with agents, list, get, delete, add / list / put / delete agent, find by key hash, look up an agent) with a Postgres implementation; duplicates are handled by translating Postgres error `23505` into a 409, not by check-then-insert
- [ ] `POST /api/projects` `{name, agents?}` returns `201` with `id`, `name`, `key` (the only time the key appears), `agents`; all-or-nothing in one transaction
- [ ] `GET /api/projects` (with `agent_count`) and `GET /api/projects/{id}` (never include the key or hash)
- [ ] `DELETE /api/projects/{id}` (`default` is protected: 409)
- [ ] Agent registry endpoints
  - [ ] `POST /api/projects/{id}/agents` `{name, url}`
  - [ ] `GET /api/projects/{id}/agents`
  - [ ] `PUT /api/projects/{id}/agents/{name}` `{url}` (create or replace; optional)
  - [ ] `DELETE /api/projects/{id}/agents/{name}`
- [ ] Validation
  - [ ] Project name: trimmed, 1 to 64 chars, unique ignoring case
  - [ ] Agent name: `^[a-z0-9][a-z0-9-]{0,31}$`; reject reserved words (`api`, `debug`)
  - [ ] Upstream: http/https with a host, no credentials, no query or fragment (private and metadata address blocking comes before any hosted deployment)
  - [ ] Name unique within a project
  - [ ] Body at most 1 MB, unknown fields rejected
- [ ] JSON error body `{"error":{"code","message"}}` on these routes (brought forward from Phase 6)
- [ ] Seed the `default` project at startup (insert if missing, never overwrite; agents from `NEWS_URL` / `FINANCIAL_URL` so the current news/financial test setup keeps working)
- [ ] Test with the curl checklist in the schema doc

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
- [ ] Decide what happens to a deleted project's in-memory groups (age out through cleanup, or remove at once)
- [ ] Verify: two projects sending the same `X-Trace-Id` end up in separate groups, and neither can read the other's

## Phase 6: hardening and config

- [ ] Flags or env for `-idle-gap`, `-retention`, `-cleanup-every`, port
- [ ] JSON error body for unknown group (the project routes already have it from Phase 3)
- [ ] CORS wrapper for a UI on another port
- [ ] Auth on the management API (`POST /api/projects` is open for now; fine on localhost, not hosted)
- [ ] Optional `.UTC()` on timestamps in the existing group views (the new project endpoints use UTC from the start)
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

- [ ] **Decide the deployment target.** Leaning: ECR for images, ECS Fargate for the proxy, RDS for Postgres (it matches existing AWS skills; the image is tiny so registry cost is negligible). The cheap alternative is a VPS running the compose file. The rest of this phase follows from the choice.
- [ ] `GET /api/health` on the proxy (liveness for the platform and for the Compose healthcheck); consider pinging the database
- [ ] ECR repository and an IAM role that GitHub Actions assumes through OIDC, limited to the `staging` and `production` branches
- [ ] `deploy.yml` workflow: build the image, tag it with the commit SHA plus the branch name, push to ECR; release tags `v*` get version tags
- [ ] Secrets (database URL, deploy settings) in the platform's secret store (for example AWS Secrets Manager or SSM), never in the repo
- [ ] Deploy job: update the ECS service to the new image, run migrations, wait for `/api/health`
- [ ] Rollback: keep the previous image tags and document how to redeploy one
- [ ] Optional: publish proxy binaries (GoReleaser) for people who want to run it without Docker, since the proxy is meant to be open source; optionally also publish a public image to GitHub Container Registry
- [ ] Only if the target is compose-based: `compose.staging.yaml` and `compose.prod.yaml` as overrides on `compose.yaml`
  - [ ] Proxy image is a pinned tag from the registry (no build from source)
  - [ ] Postgres port not published; only the proxy can reach it
  - [ ] `restart: unless-stopped`, resource limits, quieter logging
  - [ ] Secrets from the secret store or untracked env files, never committed
  - [ ] Prod only: a TLS reverse proxy (for example Caddy) in front of the proxy
- [ ] If the target is a platform (ECS, Fly.io, Cloud Run, ...): there is no prod compose file; the same per-environment settings go into the platform's config instead
- [ ] Staging environment, if it turns out to be needed before production

## Later

- [ ] Portal UI (project switcher first, then groups, then call detail)
- [ ] Streaming support (`peekBody` and duration logic assume non-streaming)
- [ ] Move groups and calls to Postgres for history, if wanted
- [ ] Hosted deployment hardening: TLS, real auth, private-address blocking (the pipeline itself is Phase 8)
- [ ] Key rotation: `POST /api/projects/{id}/rotate-key`
- [ ] Optional Kubernetes learning track: manifests in `deploy/k8s/`, run on `kind` locally; a time-boxed EKS session (bring a cluster up, deploy from ECR, tear it down the same day). Real staging and production stay on ECS.
