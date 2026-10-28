# Projects API: schema (Phase 3)

Status: design; Phase 3 is under way, starting with unit tests for the pure functions (section 11). Written 2026-10-26, updated 2026-10-27.
Scope: the management API for projects and their agent registry. Proxy behaviour (reading `X-Observer-Key`, routing by registry) is Phase 4.

## 1. Concepts

- **Project**: one app or team whose agents are observed. Has a public ID and a secret key. Never the same value as a group ID.
- **Agent**: a name plus an upstream URL, owned by exactly one project. The name is also the first path segment on the proxy (`/{name}/...`), which is why some names are reserved.
- **`default` project**: created at startup. Proxy traffic with no `X-Observer-Key` is attributed to it. It has no key.
- **Two kinds of auth, don't mix them up**: `X-Observer-Key` identifies a project on *proxy traffic* (Phase 4). The management API in this doc is open for now (fine on localhost; auth is Phase 6).

## 2. Identifiers and keys

| | Format | Visibility | Stored as |
|---|---|---|---|
| Project ID | `prj_` + 12 random lowercase letters/digits, e.g. `prj_k3x9q2m7ab4d` (hex or base32, your pick) | Public. Appears in URLs. | plain text, primary key |
| Project key | `obs_` + 32 random bytes, base64url (43 chars) | Secret. Returned once, in the create response. | SHA-256 hash (hex) plus the last 4 characters as `key_hint` |
| `default` project | id is the literal `default` | | `key_hash` is NULL, so no key can ever authenticate as it |

- Both come from `crypto/rand` (Python: `secrets`). Never `math/rand`.
- Why SHA-256 and not bcrypt: the key is 256 random bits, so there is nothing to brute-force, and bcrypt would add tens of milliseconds to every proxied request. Hashing is only there so a database leak doesn't leak working keys.
- The `obs_` prefix makes leaked keys easy to spot (secret scanners, grep).

## 3. Data (Postgres)

**projects**

| Column | Type | Notes |
|---|---|---|
| id | text, primary key | `prj_...` or `default` |
| name | text, not null | 1 to 64 chars, trimmed |
| key_hash | text, unique, nullable | NULL only for `default` (Postgres allows many NULLs under a unique constraint) |
| key_hint | text, nullable | last 4 chars of the key, so people can tell keys apart |
| created_at | timestamptz, not null, default now() | |

Plus a unique index on `lower(name)`.

**project_agents**

| Column | Type | Notes |
|---|---|---|
| project_id | text, not null, foreign key to projects(id) **on delete cascade** | |
| name | text, not null | |
| upstream_url | text, not null | |
| created_at | timestamptz, not null, default now() | |
| | primary key (project_id, name) | the same agent name can exist in two projects |

## 4. Endpoints

| Method | Path | Success | Errors |
|---|---|---|---|
| POST | `/api/projects` | 201 | 400, 409 |
| GET | `/api/projects` | 200 | |
| GET | `/api/projects/{id}` | 200 | 404 |
| DELETE | `/api/projects/{id}` | 204 | 404, 409 |
| POST | `/api/projects/{id}/agents` | 201 | 400, 404, 409 |
| GET | `/api/projects/{id}/agents` | 200 | 404 |
| PUT | `/api/projects/{id}/agents/{name}` | 200 (or 201 if new) | 400, 404 |
| DELETE | `/api/projects/{id}/agents/{name}` | 204 | 404 |

`PUT` (change an agent's URL without delete-and-add) and `DELETE /api/projects/{id}` are additions to the TODO; skip them if you want to stay on plan.

### Agent object

```json
{
  "name": "news",
  "url": "http://host.docker.internal:8001",
  "proxy_path": "/news/",
  "created_at": "2026-10-26T19:30:00Z"
}
```

`proxy_path` is derived (`"/" + name + "/"`), not stored. The full proxy URL depends on the request host (Phase 4), so it is not returned here.

### POST /api/projects

Request. `agents` is optional and defaults to none:

```json
{
  "name": "Research assistant",
  "agents": [
    { "name": "news", "url": "http://host.docker.internal:8001" }
  ]
}
```

Response `201`, with header `Location: /api/projects/prj_k3x9q2m7ab4d`. **This is the only response that ever contains `key`.**

```json
{
  "id": "prj_k3x9q2m7ab4d",
  "name": "Research assistant",
  "key": "obs_...43 characters...",
  "key_hint": "x7Qa",
  "created_at": "2026-10-26T19:30:00Z",
  "agents": [
    { "name": "news", "url": "http://host.docker.internal:8001", "proxy_path": "/news/", "created_at": "2026-10-26T19:30:00Z" }
  ]
}
```

Creation is all-or-nothing: if any listed agent is invalid, the whole request is a 400 and nothing is created (one database transaction).

### GET /api/projects

Same envelope style as `/api/groups`. Oldest first, so `default` is on top. Optional `?limit=` (default 50, max 200).

```json
{
  "total": 2,
  "projects": [
    { "id": "default", "name": "Default", "key_hint": null, "agent_count": 2, "created_at": "..." },
    { "id": "prj_k3x9q2m7ab4d", "name": "Research assistant", "key_hint": "x7Qa", "agent_count": 1, "created_at": "..." }
  ]
}
```

Group counts, running/failed counts and last activity join this list in Phase 5.

### GET /api/projects/{id}

Same shape as the create response, **without `key`**, and with the full `agents` list. Never includes the hash.

### DELETE /api/projects/{id}

`204`, no body. Agents are removed by the cascade. `default` cannot be deleted: `409 default_protected`.

### POST /api/projects/{id}/agents

Request `{ "name": "financial", "url": "http://host.docker.internal:8002" }`, response `201` with the agent object. A name that already exists in this project is `409 agent_exists`.

### GET /api/projects/{id}/agents

```json
{ "total": 1, "agents": [ { "name": "news", "url": "...", "proxy_path": "/news/", "created_at": "..." } ] }
```

### PUT /api/projects/{id}/agents/{name}

Request `{ "url": "http://host.docker.internal:8003" }`. Creates the agent if missing (`201`), replaces the URL if present (`200`). The name comes from the path, and a `name` in the body is rejected as an unknown field.

### DELETE /api/projects/{id}/agents/{name}

`204`, or `404 agent_not_found`.

## 5. Validation

| Field | Rule |
|---|---|
| Project name | trimmed, 1 to 64 **characters** (not bytes), no control characters (newline, tab, ...), unique ignoring case (`409 name_taken`) |
| Agent name | matches `^[a-z0-9][a-z0-9-]{0,31}$`; not one of the reserved words `api`, `debug`. **Case-sensitive: uppercase is rejected, never lowercased for you.** |
| Agent URL | at most 2048 chars; parses; scheme is `http` or `https`; host is not empty; no user:password; no query; no fragment; a trailing `/` is trimmed; surrounding whitespace is rejected, not trimmed |
| Request body | at most 1 MB (`413` beyond); unknown fields rejected; must be valid JSON |

Decisions behind those rules:

- **Why agent names are lowercase-only and rejected, not normalised:** the name is the first path segment on the proxy, and URL paths are case-sensitive. If `News` were silently stored as `news`, the name you registered would not match the path you typed.
- A trailing dash in an agent name (`news-`) is allowed, because the pattern allows it and it is harmless in a URL. The test cases record this.
- Project names are counted with `utf8.RuneCountInString`. `len()` counts bytes, so 64 accented characters (128 bytes) must still be valid.
- URL port range (`:99999`) is not checked: `url.Parse` accepts it. A possible later rule.
- Private and metadata address blocking is deliberately **not** here. `localhost` and `host.docker.internal` are the legitimate dev case. It comes before any hosted deployment (SSRF, see TODO Phase 8 and Later).
- Python mapping: rejecting unknown fields is Pydantic's `extra="forbid"`; the body limit is a request-size cap you would set in your web server.

## 6. Errors

One shape for every error from these endpoints (this brings the Phase 6 "JSON error body" item forward for the new routes):

```json
{ "error": { "code": "agent_exists", "message": "agent \"news\" already exists in this project" } }
```

| Status | Codes |
|---|---|
| 400 | `invalid_json`, `invalid_name`, `invalid_agent_name`, `reserved_agent_name`, `invalid_url` |
| 404 | `project_not_found`, `agent_not_found` |
| 409 | `name_taken`, `agent_exists`, `default_protected` |
| 413 | `body_too_large` |
| 500 | `internal` (log the real error, return a generic message) |

For duplicates, don't check-then-insert. Insert and translate Postgres error `23505` (unique violation) into the 409. Two simultaneous requests can both pass a check; only the constraint is race-free.

The validators return distinct error values (for example `ErrInvalidAgentName`, `ErrReservedAgentName`), so a handler can map each one to its code with `errors.Is`.

## 7. Behaviour to be aware of

- An agent added or removed applies to the next proxied request (Phase 4 reads the registry; its cache must be invalidated on every write here).
- Deleting a project leaves its in-memory groups to age out through cleanup. Phase 5 decides whether they should disappear immediately.
- `default` agents are seeded at startup from `NEWS_URL` / `FINANCIAL_URL`: **insert if missing, never overwrite**, so edits made through the API survive a restart.
- Timestamps are RFC 3339 in UTC (`.UTC()`), for the new endpoints from the start.

## 8. Suggested layout (a suggestion, your call)

```
internal/projects/   project.go        types (Project, Agent)
                     validate.go       ValidateProjectName, ValidateAgentName, ValidateUpstreamURL
                     validate_test.go
                     keys.go           NewProjectID, NewKey, HashKey, KeyHint
                     keys_test.go
                     store.go          (later) the Store interface
                     postgres.go       (later) the pgx implementation
internal/api/        projects.go       (later) handlers; the handlers struct gains a store field
cmd/proxy/main.go    open the pool, run migrations, seed default, pass the store to api.Register
```

Suggested signatures, so tests and handlers fit together:

- `ValidateProjectName(raw) (cleaned string, err error)` returns the trimmed name
- `ValidateAgentName(name) error`
- `ValidateUpstreamURL(raw) (cleaned string, err error)` returns the normalised URL

Store interface, method names only: create project (with agents), list, get by id, delete, add agent, list agents, put agent, delete agent, and (needed in Phase 4) find project by key hash and look up an agent by project and name.

## 9. Suggested build order

0. **Database layer** (done): `pgx` connection with `context`, `goose` with embedded migrations, the first migration from section 3, fail-fast at startup.
1. **Pure functions, tests first** (current step): write the table-driven tests from section 11, add a stub so they compile and fail, then implement until green. No database needed, and they give CI its first real tests.
2. Store interface and the Postgres implementation, tested against the compose Postgres (this is the CI "Postgres service container" item).
3. Handlers one endpoint at a time, in table order, curl after each.
4. Seed the `default` project and its agents last.

## 10. curl checklist

- Create with no agents, with agents, with one bad agent (nothing is created)
- Create the same name twice, differing only in case (409)
- Get one: no `key`, no hash anywhere in the output
- List: `default` first, `agent_count` correct
- Add, duplicate-add (409), reserved name `api` (400), URL `ftp://x` (400), URL with a query (400)
- PUT to change a URL, PUT a new name
- Delete an agent, delete it again (404)
- Delete a project, then get it (404); delete `default` (409)
- Body over 1 MB (413), body with an unknown field (400), malformed JSON (400)

## 11. Unit test cases (pure functions)

Table-driven Go tests (Python: `pytest.mark.parametrize`), one `t.Run` per row, in `internal/projects/*_test.go`. They need no database, so `make test` and CI run them in milliseconds. Run one with `go test ./internal/projects -run TestValidateAgentName -v`. Write them first with a stub function, watch them fail, then implement until they pass. Compare errors with `errors.Is`, which also handles the "must be accepted" (`nil`) rows.

### ValidateAgentName

| Valid (no error) | Invalid |
|---|---|
| `news`, `financial`, `a`, `a1`, `my-agent`, `agent-2`, `0abc` | empty |
| exactly 32 characters | 33 characters |
| `news-` (trailing dash is allowed) | `-news` (leading dash) |
| `apis`, `api-gateway`, `debugger` (they only start like reserved words) | `News`, `API` (uppercase: format error, because the format check runs before the reserved check) |
| | `my_agent`, `a.b`, `a/b`, `my agent`, `news ` (trailing space), `néws`, `.well-known` |
| | `api`, `debug` → the **reserved-word** error, not the format error |

### ValidateProjectName (returns the trimmed name)

| Valid → result | Invalid |
|---|---|
| `Research assistant` → same | empty |
| one character | spaces only |
| exactly 64 characters | 65 characters |
| `  padded  ` → `padded` | `a\nb` (control character) |
| 64 × `é` (128 bytes, 64 characters) → accepted | |

### ValidateUpstreamURL (returns the normalised URL)

| Valid → result | Invalid |
|---|---|
| `http://localhost:8001` → same | empty |
| `https://agent.example.com` → same | `localhost:8001` (no scheme: `url.Parse` reads `localhost` as the scheme, a classic trap) |
| `http://host.docker.internal:8001` → same | `//host`, `http://` (no host), `http:///path` |
| `http://news:8001/` → `http://news:8001` (slash trimmed) | `ftp://x`, `file:///etc/passwd`, `javascript:alert(1)` |
| `http://10.0.0.5:8080/a2a` → keeps the path | `http://user:pass@host`, `http://user@host` (credentials) |
| `http://[::1]:8001` (IPv6) → same | `http://host?x=1` (query), `http://host#frag` (fragment) |
| `HTTP://localhost:8001` → `http://localhost:8001` (scheme lowercased by the parser) | `http://ho st` (space), a URL over 2048 characters |
| | `" http://x "` (surrounding whitespace is rejected, not trimmed) |

### Keys and IDs (test the shape, not the randomness)

| Function | Checks |
|---|---|
| `NewProjectID` | starts with `prj_`, 16 characters, only lowercase letters and digits; two calls return different values |
| `NewKey` | starts with `obs_`, 47 characters (4 + 43), only letters, digits, `-` and `_`; two calls differ |
| `HashKey` | same input gives the same output; different inputs differ; output is 64 lowercase hex characters; known vector: `HashKey("abc")` equals `ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad` |
| `KeyHint` | last 4 characters, for example `obs_abcdefgh` → `efgh` |

## 12. Differences from backend-todo.md Phase 3

- `projects` gains `key_hint`, a default for `created_at`, a nullable `key_hash` and the unique lowercase-name index.
- `project_agents` gains `created_at` and `on delete cascade`.
- Create accepts an optional `agents` list (as discussed).
- Added: `DELETE /api/projects/{id}`, `PUT .../agents/{name}`.
- JSON error body is pulled forward from Phase 6 for these routes.
- Project names reject control characters and are counted in characters; agent names are case-sensitive (uppercase rejected).

## 13. Not in this phase

Auth on the management API (Phase 6), key rotation (later: `POST /api/projects/{id}/rotate-key`), renaming (`PATCH`), port-range check on upstream URLs, private-address blocking, per-project group counts (Phase 5), and the UI.
