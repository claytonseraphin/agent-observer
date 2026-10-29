# agent-observer: backend TODO

Scope: backend only. The portal UI waits until the backend below is done.

Last updated: 2026-10-29 (all three validators done; key generation is next)

Companion doc: `claude/projects-api-schema.md` (the full Phase 3 endpoint design, with the unit-test case tables).

## Done

- [x] Reverse proxy that forwards A2A traffic and rewrites agent cards
- [x] Call tracking: discovery vs call kinds, outcomes (success / failed / processing), error classification
- [x] Grouping: `X-Trace-Id` when the client sends it, idle-gap inference per client address otherwise
- [x] Memory cleanup for the tracker and the group assigner
- [x] `GET /api/groups` and `GET /api/groups/{id}` (tested, including the failure path and 404)
- [x] Phase 1 code restructure into `internal/` packages (see Phase 1 for the remaining housekeeping)
- [x] Docker image, layered compose files and Makefile (Phase 2)
- [x] GitHub Actions workflow `ci.yml` is in place and running (see Phase 2b)
- [x] Database layer: `pgx` pool, embedded `goose` migrations, first migration (`projects`, `project_agents`), fail-fast startup (see Phase 2)
- [x] The three validators with table-driven unit tests: `ValidateAgentName`, `ValidateProjectName`, `ValidateUpstreamURL` (see Phase 3)

## Decisions already made

| Decision | Choice |
|---|---|
| Project identity | Own ID (public) and own key (secret). Never the group ID. |
| How the proxy learns the project | `X-Observer-Key` header (path-prefix fallback only if the Python A2A client cannot send headers) |
| Where routes come from | Per-project agent registry (name to upstream URL), registered through the API. No hardcoded `routes` map. |
| Storage | Postgres for projects and agents. Groups and calls stay in memory for now, behind an interface so they can move later. |
| Keys at rest | Hashed with SHA-256 (the key is 256 random bits, so bcrypt would only slow every proxied request). The plain key is shown once, in the create response. |
| Postgres driver | `pgx` |
| Migrations | `goose`, with the `.sql` files embedded in the binary (`internal/db/migrations/`). The same files can be run from disk with the goose CLI. |
| Database config | One `DATABASE_URL` environment variable. Compose builds it from `POSTGRES_*` with host `postgres`; `make run` builds it with host `localhost`. No config file with passwords. |
| Compose layout | Base `compose.yaml` plus per-environment overrides (dev now; staging and prod in Phase 8 once the deployment target is chosen) |
| No key on a request | Attributed to an auto-created `default` project |
| Wrong key | 401 with a clear message |
| Projects API shape | See `claude/projects-api-schema.md`: IDs `prj_...`, keys `obs_...`, JSON error body, optional agents at create |
| Agent names | `^[a-z0-9][a-z0-9_-]{0,31}$`, case-sensitive (uppercase rejected, not lowercased), reserved words `api` and `debug`. Underscores allowed inside; a leading `_` or `.` stays free for the proxy's own routes. |
| Project names | trimmed, 1 to 64 characters (counted as characters, not bytes), no control characters; one character is allowed |
| Upstream URLs | Validated, then **stored exactly as typed** (no normalisation). A realistic AWS AgentCore URL has a percent-encoded ARN (`%2F`) and a trailing slash; rebuilding the URL decoded the ARN and trimmed the slash, which would break the call. `ValidateUpstreamURL` returns only an error. Error messages never include the URL (it can carry a password). |
| Platform support | Plain A2A (self-hosted agents) stays the default and the original design. Platform-specific behaviour (AWS AgentCore, LangChain / LangGraph, others) is a per-**agent** setting, not per project, because one project can mix them. Built later, one platform at a time, only after its contract has been read. The base validator stays vendor-neutral; per-platform checks go on top. See Phase 6b. |
| Upstream credentials | Never stored in the registry. Bearer tokens pass through from the client. SigV4, if added, signs with the proxy's own IAM role. |
| Go version | 1.26. The remaining standard-library vulnerability fixes exist only in 1.26.x (1.25.14 is the last 1.25 patch). CI takes the latest patch. |
| CI branches | `staging` and `production` (no `main`). Checks on every push; Docker build and govulncheck only on those two. |
| Container registry | ECR, pushed from GitHub Actions with OIDC (no AWS keys stored in GitHub) |
| Compute | Leaning ECS Fargate. Kubernetes (kind locally, a time-boxed EKS session) is a learning track only. |

## Phase 1: restructure the folder (code done, housekeeping left)

Goal: same behaviour, new layout. Nothing new gets built in this phase.

- [x] Module path: `github.com/claytonseraphin/agent-observer`
- [x] Layout (as built):
  ```
  cmd/proxy/main.go          wiring only: connects the database and runs migrations, creates tracker + assigner, registers routes, ListenAndServe
  cmd/proxy/cleanup.go       startCleanup + the cleaner interface (needs both tracker and assigner)
  cmd/proxy/helper.go        mustParse (temporary, goes with the routes map in Phase 4)
  internal/outcome/          call_outcome.go
  internal/grouping/         client_calls.go (GroupAssigner + its Cleanup)
  internal/tracker/          tracker.go, output.go, memory_growth.go (Tracker + its Cleanup)
  internal/proxy/            proxy.go (Proxy, New, ServeHTTP, clientAddress), card.go (rewriteCard)
  internal/api/              routes.go (Register), api.go (handlers on a struct holding the tracker)
  internal/db/               db.go (Connect), migrate.go (Migrate, embedded), migrations/0001_projects.sql
  internal/projects/         validate.go, validate_test.go (Phase 3); keys.go and the rest to come
  ```
- [x] Globals replaced by injected values (`proxy.New(tr, asg, routes, publicBase)`, `api.Register(mux, tr)`)
- [x] Re-run curl tests: success, failed discovery, 404, list, cleanup. Output identical.
- [ ] Remove the temporary `assigner clients=%d` debug log
- [ ] Restore production cleanup values: `startCleanup(time.Minute, time.Hour, tr, asg)`
- [ ] Move the loose `.md` / `.pdf` files into `docs/`
- [ ] Confirm `.env` and `.DS_Store` are listed in `.gitignore`
- [ ] Always run from the repo root: `go run ./cmd/proxy`

## Phase 2: Docker and Postgres

Order: the Docker and compose items are done, Phase 2b (CI) is mostly done, and the database layer is done. Phase 3 is under way.

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
- [x] `Makefile` with dev targets: `help`, `dev`, `down`, `reset`, `logs`, `ps`, `psql`, `check` (gofmt + vet + build), `test`, `run` (builds the host `DATABASE_URL` from `.env`). Staging and prod targets come with their compose files in Phase 8.
- [ ] Gotcha to handle: inside the container, `localhost:8001` is the container, not your Mac. Registered upstreams for host-run agents use `http://host.docker.internal:8001`. Document this in the README.

### Database layer (done)

- [x] Learned `context.Context` (rules of thumb: `ctx` first parameter; `context.Background()` only at the top; `r.Context()` in handlers; `WithTimeout` plus `defer cancel()`)
- [x] `pgx` added
- [x] `internal/db/db.go`: `Connect(ctx, url)` opens a `pgxpool` pool and pings with a 5-second timeout; errors never print the password
- [x] `DATABASE_URL` read in `main` and required. Inside compose the host is `postgres`; for `make run` on the Mac it is `localhost:5432`.
- [x] `goose` added; `.sql` files embedded from `internal/db/migrations/` and run at startup before the server listens (`internal/db/migrate.go`)
- [x] First migration `0001_projects.sql`
- [x] Startup order in `main`: connect (fail fast), migrate, then the rest
- [x] Verified: tables exist after `make dev`, a restart does not re-run migrations, and the proxy still works through port 8080
- [ ] Optional, and a good use of context: graceful shutdown with `signal.NotifyContext` and `http.Server.Shutdown` (ECS sends SIGTERM when it stops a task)
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
- [x] First tests so CI has something to run: the three validators (pure functions, no database). More to come: key generation, `GroupAssigner`, `outcomeFromRPCBody`, `groupIsOld`, `Snapshot`.
- [ ] Branch protection on **`staging` and `production`** (not `main`): pull request required, check "Format, vet, build, test" required; on `production` also "Docker image builds" and branch up to date; vulncheck stays optional; block force pushes and deletions. Private repos need a paid GitHub plan for this to be enforced.
- [ ] Optional: add a `pull_request` trigger limited to `staging` and `production`, so the Docker build can block a merge (as it stands, it runs after the merge)
- [ ] Postgres service container in the workflow, for the store tests (once Phase 3 adds them)
- [ ] When CD arrives: stop cancelling in-progress runs on `production` (`cancel-in-progress` is on for CI only)

## Phase 3: projects endpoint and agent registry

Full design: `claude/projects-api-schema.md` (data, request and response shapes, validation, errors, curl checklist, unit-test case tables). The registry is part of the project record, so these are built together.

- [ ] Pure functions first, with table-driven unit tests (tests written before the function; rejected and accepted inputs in separate tables):
  - [x] `ValidateAgentName`
  - [x] `ValidateProjectName`
  - [x] `ValidateUpstreamURL` (returns only an error; the URL is stored as typed, so an AgentCore URL with an encoded ARN survives)
  - [ ] Project and key generation (`crypto/rand`): `NewProjectID`, `NewKey`, `HashKey` (SHA-256), `KeyHint`. Start with `HashKey` (known vector for `"abc"`), then the shape checks.
- [ ] `ProjectStore` interface (create with agents, list, get, delete, add / list / put / delete agent, find by key hash, look up an agent) with a Postgres implementation; duplicates are handled by translating Postgres error `23505` into a 409, not by check-then-insert
- [ ] `POST /api/projects` `{name, agents?}` returns `201` with `id`, `name`, `key` (the only time the key appears), `agents`; all-or-nothing in one transaction
- [ ] `GET /api/projects` (with `agent_count`) and `GET /api/projects/{id}` (never include the key or hash)
- [ ] `DELETE /api/projects/{id}` (`default` is protected: 409)
- [ ] Agent registry endpoints
  - [ ] `POST /api/projects/{id}/agents` `{name, url}`
  - [ ] `GET /api/projects/{id}/agents`
  - [ ] `PUT /api/projects/{id}/agents/{name}` `{url}` (create or replace; optional)
  - [ ] `DELETE /api/projects/{id}/agents/{name}`
- [ ] Validation wiring
  - [ ] Name unique within a project
  - [ ] Body at most 1 MB, unknown fields rejected
  - [ ] Private and metadata address blocking comes before any hosted deployment
- [ ] JSON error body `{"error":{"code","message"}}` on these routes (brought forward from Phase 6). Error messages never include the submitted URL (it can contain a password).
- [ ] Seed the `default` project at startup (insert if missing, never overwrite; agents from `NEWS_URL` / `FINANCIAL_URL` so the current news/financial test setup keeps working)
- [ ] Test with the curl checklist in the schema doc
- [x] Schema doc updated for the validator decisions (underscore rule, project-name rules, URL stored as typed, test tables)

## Phase 4: handler uses the registry

- [ ] Read `X-Observer-Key`: missing means `default`; unknown means 401
- [ ] Cache key-to-project lookups in memory (invalidate on writes) so each proxied request does not hit Postgres
- [ ] First path segment is the agent name, looked up in the project's agents; unknown agent gives a 404 that names it
- [ ] Build the ReverseProxy for that upstream (share one transport)
- [ ] **Outgoing `Host` header must be the upstream's host.** Local agents ignore it, but a remote HTTPS upstream (AWS and similar) may reject a request whose `Host` still says the proxy. Check how the outgoing request is built, and test against a remote HTTPS upstream.
- [ ] **Trailing slash:** upstream URLs are stored as typed, so confirm the path join gives a valid request path whether or not the stored URL ends in `/` (for example `.../invocations/` and `.../invocations`)
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
- [ ] Optional `MIGRATE_ON_START` toggle, or goose's advisory lock, before running more than one proxy instance (migrations run at startup today, which is fine for one instance)

## Phase 6b: platform connectors (optional, after the core backend)

Idea: keep the original design for plain A2A agents (the news and financial projects) and let each **agent** declare where it is hosted, so the same proxy can observe agents on AWS, LangChain / LangGraph or other platforms that speak A2A. The proxy works at the protocol level, so most of the design is unchanged. What differs between platforms is small: URL shape, upstream authentication, extra headers, streaming, and the A2A card version. A2A standardises the messages, not where an agent lives or how you authenticate to it.

Adoption is wide (Google Cloud / Vertex AI, Microsoft Azure AI Foundry and Copilot Studio, Salesforce Agentforce, IBM BeeAI, LangChain and others, per a quick look on 2026-10-28). That list is evidence the protocol has traction, **not a backlog**: a connector is built only when there is an agent actually running on that platform. My guess, to be verified per platform, is that most clouds use OAuth-style bearer tokens, which pass through; AWS SigV4 would be the outlier.

Plan:

- [ ] Add an optional `kind` on the agent (default `a2a` = plain, today's behaviour). Stored on `project_agents` with a new migration (`0002`), returned and accepted by the agents API. Per agent, not per project.
- [ ] One small "upstream" step per kind that prepares the outgoing request (host, headers, signing). Plain A2A does nothing, so existing behaviour is untouched.
- [ ] Optional per-kind URL check on top of the common validator (for `aws_agentcore`: host `bedrock-agentcore.<region>.amazonaws.com` and a path shaped like `/runtimes/<encoded ARN>/invocations/`), giving a clear error when someone pastes the wrong URL. The common validator stays vendor-neutral.
- [ ] Add a platform only after its A2A contract has been read. **Only AWS AgentCore has been looked at so far.**
- [ ] Keep a compatibility table, one row per platform: card location, URL shape, auth method, streaming, extra headers, A2A version. It shows which platforms need code and which need none.
- [ ] Collect a real agent card from each platform as a test fixture for the card rewrite (it already handles `url`, `supportedInterfaces[].url` and `additionalInterfaces[].url`).
- [ ] Platforms to research when the time comes: Google Cloud (Vertex AI / Gemini Enterprise agent platform), Microsoft (Azure AI Foundry, Copilot Studio), Salesforce Agentforce, IBM (BeeAI; its Agent Communication Protocol was merged into A2A, verify), LangChain / LangGraph, and others.
- [ ] Upstream credentials are never stored in the registry (see Decisions).

AWS AgentCore Runtime (`kind: aws_agentcore`), from the AWS docs read on 2026-10-28:

- A2A agents run as containers on port 9000: `POST /` (JSON-RPC), `GET /.well-known/agent-card.json`, `GET /ping`.
- Clients call `https://bedrock-agentcore.<region>.amazonaws.com/runtimes/<URL-encoded ARN>/invocations/`; the card is at that URL plus `/.well-known/agent-card.json`; the card's `url` is the invocation URL (so the existing card rewrite applies).
- Authentication is OAuth 2.0 bearer or SigV4 (whichever the agent was configured with). Requests carry `X-Amzn-Bedrock-AgentCore-Runtime-Session-Id`.
- Errors use real HTTP status codes with a JSON-RPC error body (custom codes `-32051` to `-32055`, `-32603`), instead of the usual HTTP 200 with an error.
- The card advertises `streaming: true`.

Tasks for it:

- [x] `ValidateUpstreamURL` keeps the encoded ARN (done in Phase 3: URLs are stored as typed)
- [ ] Bearer tokens: verify they pass through the proxy untouched, against a real deployed agent
- [ ] Verify the trailing slash on `/invocations/` (does the POST work without it?), together with the Phase 4 join check
- [ ] Outcome classification: make sure non-2xx responses with a JSON-RPC error body are classified as failed, and that a retryable conflict (`-32054`) followed by a retry reads sensibly in a group
- [ ] Use the `X-Amzn-Bedrock-AgentCore-Runtime-Session-Id` header as a grouping hint when no `X-Trace-Id` is sent
- [ ] Streaming (`message/stream`): depends on the streaming item in Later
- [ ] **SigV4 is the hard one.** A client's signature covers the host and path, so a request signed for the proxy is rejected once forwarded to AWS. Supporting it means the proxy re-signs requests with its own AWS credentials (an ECS task role, with the AWS SDK signer), which makes it an authenticated gateway. Treat as its own feature, after bearer pass-through works.
- [ ] Test the whole path against a real deployed AgentCore agent

Sources: [A2A protocol contract](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/runtime-a2a-protocol-contract.html), [Deploy A2A servers in AgentCore Runtime](https://docs.aws.amazon.com/bedrock-agentcore/latest/devguide/runtime-a2a.html).

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
- [ ] Database URL for RDS uses a TLS mode (`sslmode=require` or stricter), unlike the local `sslmode=disable`
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
- [ ] Before taking the project public or commercial: choose a license on purpose (get legal advice), check the project name for trademark conflicts, keep the privacy defaults (no raw prompts stored), and talk to a handful of people building multi-agent systems
- [ ] Optional Kubernetes learning track: manifests in `deploy/k8s/`, run on `kind` locally; a time-boxed EKS session (bring a cluster up, deploy from ECR, tear it down the same day). Real staging and production stay on ECS.
