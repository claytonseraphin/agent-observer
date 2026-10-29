# Projects API: schema (Phase 3)

Status: design; Phase 3 is under way. The three validators (agent name, project name, upstream URL) are written and tested; key generation, the store and the handlers are next. Written 2026-10-26, updated 2026-10-29.
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
| name | text, not null | 1 to 64 characters, trimmed |
| key_hash | text, unique, nullable | NULL only for `default` (Postgres allows many NULLs under a unique constraint) |
| key_hint | text, nullable | last 4 chars of the key, so people can tell keys apart |
| created_at | timestamptz, not null, default now() | |

Plus a unique index on `lower(name)`.

**project_agents**

| Column | Type | Notes |
|---|---|---|
| project_id | text, not null, foreign key to projects(id) **on delete cascade** | |
| name | text, not null | |
| upstream_url | text, not null | stored exactly as typed (see section 5) |
| created_at | timestamptz, not null, default now() | |
| | primary key (project_id, name) | the same agent name can exist in two projects |

A later migration (Phase 6b, optional) adds a `kind` column for platform-specific agents; plain A2A stays the default.

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
| Agent name | matches `^[a-z0-9][a-z0-9_-]{0,31}$`; not one of the reserved words `api`, `debug`. **Case-sensitive: uppercase is rejected, never lowercased for you.** |
| Agent URL | at most 2048 chars; no leading or trailing whitespace; parses; scheme is `http` or `https`; the host name is not empty; no user:password (nor a username alone); no query and no fragment (a bare `?` or `#` counts). **Stored exactly as typed: nothing is rewritten.** |
| Request body | at most 1 MB (`413` beyond); unknown fields rejected; must be valid JSON |

Decisions behind those rules:

- **Why agent names are lowercase-only and rejected, not normalised:** the name is the first path segment on the proxy, and URL paths are case-sensitive. If `News` were silently stored as `news`, the name you registered would not match the path you typed.
- **Underscores are allowed inside an agent name (`news_agent`)**, because they are harmless in a URL path and Python developers write them naturally. A name must still start with a letter or digit, so a leading `_` or `.` stays free for the proxy's own routes (for example `/_health`) and can never collide with an agent. A trailing dash (`news-`) is allowed. If a name ever has to become a hostname (subdomain routing), underscores would have to be revisited.
- Project names are counted with `utf8.RuneCountInString`. `len()` counts bytes, so 64 accented characters (128 bytes) must still be valid. A single-character project name is allowed.
- **Why the upstream URL is stored exactly as typed:** the first version rebuilt the URL after parsing (lowercase scheme, trailing slash trimmed). A realistic AWS AgentCore URL (`.../runtimes/arn%3Aaws%3A...%3Aruntime%2Fname/invocations/`) came back with `%3A` and `%2F` decoded, which turned an encoded `/` into a real path separator, and with the trailing slash removed. Rewriting the path is never needed (the proxy parses the URL again anyway, and the unique key is the project plus the agent name), so the validator now only *checks* and returns an error. Scheme case is accepted either way (`HTTP://...` passes), but left as typed.
- Error messages never include the submitted URL: it can contain a password, and the messages end up in API responses and logs. A parse failure does not wrap the underlying `*url.Error`, which repeats the whole URL.
- URL port range (`:99999`) is not checked: `url.Parse` accepts it. A possible later rule.
- Private and metadata address blocking is deliberately **not** here. `localhost` and `host.docker.internal` are the legitimate dev case. It comes before any hosted deployment (SSRF, see TODO Phase 8 and Later).
- Per-platform URL checks (for example, an AWS AgentCore agent must look like `bedrock-agentcore.<region>.amazonaws.com/runtimes/<ARN>/invocations/`) are an optional second layer on top of this common validator, planned for Phase 6b.
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

The validators return distinct error values (`ErrInvalidAgentName`, `ErrReservedAgentName`, `ErrInvalidProjectName`, `ErrInvalidUpstreamURL`). A handler maps each one to its code with `errors.Is`. The URL validator wraps its sentinel with the reason (`fmt.Errorf("%w: scheme must be http or https", ...)`), so `errors.Is` still matches while the message says what was wrong.

## 7. Behaviour to be aware of

- An agent added or removed applies to the next proxied request (Phase 4 reads the registry; its cache must be invalidated on every write here).
- Deleting a project leaves its in-memory groups to age out through cleanup. Phase 5 decides whether they should disappear immediately.
- `default` agents are seeded at startup from `NEWS_URL` / `FINANCIAL_URL`: **insert if missing, never overwrite**, so edits made through the API survive a restart.
- Timestamps are RFC 3339 in UTC (`.UTC()`), for the new endpoints from the start.
- Because upstream URLs are stored as typed, whether a trailing slash matters depends on how the proxy joins the request path to the upstream path. Phase 4 verifies this, including against a remote HTTPS upstream.

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

Signatures, as built:

- `ValidateProjectName(raw) (cleaned string, err error)` returns the trimmed name
- `ValidateAgentName(name) error`
- `ValidateUpstreamURL(raw) error` (the URL is stored as typed, so there is nothing to return)

Store interface, method names only: create project (with agents), list, get by id, delete, add agent, list agents, put agent, delete agent, and (needed in Phase 4) find project by key hash and look up an agent by project and name.

## 9. Suggested build order

0. **Database layer** (done): `pgx` connection with `context`, `goose` with embedded migrations, the first migration from section 3, fail-fast at startup.
1. **Pure functions, tests first**: the three validators are done. Key and ID generation is next (section 11).
2. Store interface and the Postgres implementation, tested against the compose Postgres (this is the CI "Postgres service container" item).
3. Handlers one endpoint at a time, in table order, curl after each.
4. Seed the `default` project and its agents last.

## 10. curl checklist

- Create with no agents, with agents, with one bad agent (nothing is created)
- Create the same name twice, differing only in case (409)
- Get one: no `key`, no hash anywhere in the output
- List: `default` first, `agent_count` correct
- Add, duplicate-add (409), reserved name `api` (400), URL `ftp://x` (400), URL with a query (400)
- Add an agent whose URL contains an encoded ARN (`%2F`): the URL comes back unchanged
- PUT to change a URL, PUT a new name
- Delete an agent, delete it again (404)
- Delete a project, then get it (404); delete `default` (409)
- Body over 1 MB (413), body with an unknown field (400), malformed JSON (400)

## 11. Unit test cases (pure functions)

Table-driven Go tests (Python: `pytest.mark.parametrize`), one `t.Run` per row, in `internal/projects/*_test.go`. They need no database, so `make test` and CI run them in milliseconds. Run one with `go test ./internal/projects -run TestValidateAgentName -v`.

Conventions we settled on:

- **Tests first.** Write the tests with a stub that compiles, watch them fail, then implement until green.
- **Rejected and accepted inputs go in separate tables.** A rejected table has `name`, `input` (and `wantErr` when more than one error is possible); an accepted table has `name`, `input`, and `want` when the function returns a value. Struct fields that are always empty are dropped, not filled with placeholders.
- **Compare errors with `errors.Is`** (it also handles "no error expected", and it matches wrapped errors). Print `tc.wantErr` in the message, never a hard-coded error.
- **Groups of inputs with one expected result use a slice and a loop**: the reserved words, the words that only start like a reserved word (`apis`, `api-gateway`, `debugger`), the disallowed URL schemes. The test keeps its own list of reserved words, separate from the one in the implementation, so removing a word from the code is caught.
- Build long inputs with `strings.Repeat`; declare a shared value above the table when input and expected output are the same string.
- Run `gofmt -w` on the package before `make check`.

### ValidateAgentName

| Valid (no error) | Invalid |
|---|---|
| `news`, `financial`, `a`, `n`, `0abc`, `my-agent`, `agent-2` | empty |
| `news_agent` (underscore inside) | `_news` (leading underscore) |
| `news-` (trailing dash is allowed) | `-news` (leading dash) |
| exactly 32 characters | 33 characters |
| `apis`, `api-gateway`, `debugger` (they only start like reserved words) | `News`, `API` (uppercase: format error, because the format check runs before the reserved check) |
| | `.news`, `news.agent`, `/news`, `news/agent`, `news agent`, `news ` (trailing space), `néws`, `.well-known`, `news\n` (trailing newline: Go's `$` matches only at the very end of the text, unlike Python's) |
| | `api`, `debug` → the **reserved-word** error, not the format error |

### ValidateProjectName (returns the trimmed name)

| Valid → result | Invalid |
|---|---|
| `Research assistant`, `project 2` → same | empty |
| `a` (one character) → same | spaces only |
| exactly 64 characters → same | 65 characters, and 65 × `é` |
| `  padded  ` → `padded`; `project ultimate\n` → `project ultimate`; `\tproject` → `project` | `a\nb` (control character inside) |
| `a  b` (two spaces) → `a  b` (only the ends are trimmed) | |
| 64 × `é` (128 bytes, 64 characters) → same | |
| `  ` + 64 characters + `  ` → the 64 characters (length counted after trimming) | |
| `Équipe spéciale` → same | |

### ValidateUpstreamURL (returns only an error; nothing is rewritten)

| Valid (no error) | Invalid |
|---|---|
| `http://localhost:8001`, `https://agent.example.com`, `http://host.docker.internal:8001` | empty |
| `http://news:8001/` (trailing slash kept) | `http://a space` (space in the host), `" http://x  "` (surrounding whitespace) |
| `http://10.0.0.5:8080/a2a`, `http://host/a2a/` | a URL of 2049 characters |
| `http://[::1]:8001` (IPv6) | `http://host?x=1`, `http://host?` (query, even an empty one) |
| `HTTP://localhost:8001` (scheme accepted, left as typed) | `http://host#frag`, `http://host#` (fragment, even an empty one) |
| exactly 2048 characters | `http://user:pass@host`, `http://user@host` (credentials; the second catches a check that only looks for a password) |
| an AgentCore URL with an encoded ARN (`...arn%3Aaws%3A...%3Aruntime%2Fname/invocations/`) | `http://`, `http:///path`, `http://:8001` (no host name) |
| | scheme group (a slice loop): `example.com` (no scheme), `//host` (empty scheme), `localhost:8001` (`localhost` is parsed as the scheme), `ftp://x`, `file:///etc/passwd`, `javascript:alert(1)` |

Also test that an error message never contains the submitted password (`http://user:secret-password@host`).

### Keys and IDs (next; test the shape, not the randomness)

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
- Project names reject control characters and are counted in characters; agent names are case-sensitive (uppercase rejected) and may contain underscores inside.
- Upstream URLs are validated but stored as typed (no normalisation), and `ValidateUpstreamURL` returns only an error.

## 13. Not in this phase

Auth on the management API (Phase 6), key rotation (later: `POST /api/projects/{id}/rotate-key`), renaming (`PATCH`), port-range check on upstream URLs, private-address blocking, per-platform (`kind`) URL checks and connectors (Phase 6b), per-project group counts (Phase 5), and the UI.
