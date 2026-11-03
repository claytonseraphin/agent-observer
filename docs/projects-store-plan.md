# ProjectStore: files, test setup, test cases, implementation

Phase 3 step 2 of `claude/backend-todo.md`. Companion to `claude/projects-api-schema.md`.
Written 2026-11-01. First slice: `CreateProject`, then `GetProject`. Tests are written before the code, as with the pure functions.

## 1. Decisions

| Decision | Choice |
|---|---|
| Interface | `ProjectStore` in `store.go`, with the five sentinel errors beside it. It does not import `pgxpool`. |
| Real implementation | `PostgresStore` in `postgres.go`, holding the one shared `*pgxpool.Pool` (created once in `main` by `db.Connect`). |
| What the store receives | A ready-made `Project` (ID, name, key hash and hint already set, plus its agents). The store has no randomness and never sees the plain key. |
| Domain vs response structs | `Project` and `Agent` (in `project.go`) have no `json` tags. Each endpoint gets its own response struct in `internal/api`, so `KeyHash` can never leak. |
| Testing the store | Against a **real** Postgres. A mock cannot check the SQL, the constraint names, the `23505` translation or the rollback. |
| Testing the handlers | Later, in `internal/api`, with a small hand-written fake store (a struct with a map). The fake returns the same sentinel errors as the real store. |
| Growth | The interface starts with `CreateProject` only. Each method is added with its tests, one at a time. |

## 2. Files

```
internal/projects/
  project.go            Project, Agent (no json tags)
  store.go              ProjectStore interface + 5 sentinel errors   (imports: context, errors)
  postgres.go           PostgresStore, NewPostgresStore, the queries
  postgres_test.go      newTestStore helper + store tests (real Postgres)
internal/api/
  projects.go           (later) handlers + response structs
  projects_test.go      (later) handler tests + the fake store
cmd/proxy/main.go       (later) pool -> NewPostgresStore(pool) -> handlers
```

Sentinel errors, with the situation each one covers:

| Error | Situation | HTTP |
|---|---|---|
| `ErrProjectNotFound` | no project with that ID | 404 |
| `ErrAgentNotFound` | no agent with that name in the project | 404 |
| `ErrNameTaken` | project name already used (ignoring case) | 409 |
| `ErrAgentExists` | agent name already used in that project | 409 |
| `ErrDefaultProtected` | tried to delete `default` | 409 |

Compile-time check in `postgres.go`: `var _ ProjectStore = (*PostgresStore)(nil)`. It fails the build if the struct is missing a method of the interface.

## 3. Test setup

### Running the tests

- Go reads `DATABASE_URL` from the **process environment**. Nothing reads the Makefile or `.env` directly.
- `make test-db` sets it for one command, built from `POSTGRES_*` the same way the `run` target does, then runs `go test -v ./internal/projects/...`. It needs the compose Postgres running with port 5432 published (`make dev`).
- `make test` (and CI) do not set it, so the store tests are **skipped** and everything else still runs.
- Add `test-db` to `.PHONY` and give it a `##` description for `make help`. Its position in the file does not matter.

### The `newTestStore(t *testing.T) *PostgresStore` helper (lowercase, private to the test file)

1. `t.Helper()` first, so failures point at the calling test.
2. `databaseURL := os.Getenv("DATABASE_URL")`; if empty, `t.Skip("DATABASE_URL not set")` (no `return`, `Skip` stops the test).
3. One context with a 30-second timeout and `defer cancel()`. `Connect` already applies its own 5-second limit inside.
4. `pool, err := db.Connect(ctx, databaseURL)`; on error `t.Fatalf`.
5. `t.Cleanup(pool.Close)` right away, so the pool closes even if the next step fails.
6. `err = db.Migrate(ctx, pool)` (plain `=`, `err` already exists); on error `t.Fatalf`.
7. `return NewPostgresStore(pool)`.

Imports: `context`, `os`, `testing`, `time`, and `github.com/claytonseraphin/agent-observer/internal/db`.

### Test data and cleanup

- Every test builds its own project with a **unique ID and a unique name** (put `NewProjectID()` inside the name). The `lower(name)` index is unique, so leftovers from an earlier run would otherwise collide.
- A helper (for example `newTestProject(t)`) builds the `Project` and registers `t.Cleanup` that runs `DELETE FROM projects WHERE id = $1` through the pool (`store.pool` is reachable because the test is in the same package). The cascade removes the agents.
- Register the cleanup **before** calling `CreateProject`. Deleting a row that does not exist is harmless, and a failed create still gets cleaned up.
- Before `GetProject` exists, check what was written with raw SQL through the pool (for example `SELECT count(*) FROM projects WHERE id = $1`). Once `GetProject` exists, switch the create tests to it.
- Test function names start with `Test` (the character after it must not be lowercase). Helpers do not, so `go test` never runs them as tests. Table row names are only `t.Run` labels. Check what Go found with `go test ./internal/projects -list '.*'`.

## 4. Test cases

### CreateProject

| # | Case | Expected |
|---|---|---|
| 1 | No agents | returned project has the same ID and name, and a non-zero `CreatedAt` |
| 2 | Two agents | both are stored with names, URLs and `CreatedAt` (read back by raw SQL, then by `GetProject`) |
| 3 | Same name in a different case (`Research`, then `research`) | `errors.Is(err, ErrNameTaken)` |
| 4 | Two agents with the same name in one request | `ErrAgentExists`, **and** the project does not exist afterwards (proves the transaction rolled back) |
| 5 | Empty `KeyHash` and `KeyHint` (the `default` style) | accepted, stored as NULL, read back as empty strings; two such projects can coexist |

Plain tests, not a table: each case checks something different. A table fits only when many inputs share one check.

### GetProject (added after CreateProject is green)

| # | Case | Expected |
|---|---|---|
| 6 | Unknown ID | `errors.Is(err, ErrProjectNotFound)` |
| 7 | Hash and hint | both come back populated (the store returns the hash because Phase 4 needs it; the handler hides it) |
| 8 | Several agents | returned in a stable order, sorted by name |

Not tested: a duplicate ID or a duplicate hash. They have no sentinel (the handler would not act differently), so they come back as a generic wrapped error and become a 500.

## 5. Implementation outline

### Order of work (per method)

1. Add the method to the interface and a stub on `PostgresStore` that returns `Project{}, errors.New("not implemented")`. `go build ./...` must pass.
2. Write the tests, run `make test-db`, and watch them fail.
3. Implement until green. Then `gofmt -w`, `go vet`, `make check && make test`, commit.

### How the store talks to Postgres (no ORM)

- `*pgxpool.Pool` is a pool of connections, safe to share between goroutines. One pool for the whole program; the store holds a reference.
- Values travel as placeholders (`$1`, `$2`), never by joining strings (SQL injection).
- `Exec` (no rows back), `QueryRow(...).Scan(&a, &b)` (one row; Scan fills variables by address, in column order), `Query` (many rows), `Begin` (transaction).
- Methods use a receiver (Go's `self`): `func (s *PostgresStore) CreateProject(...)`, and call `s.pool...`.

### CreateProject

1. `tx, err := s.pool.Begin(ctx)`, then `defer tx.Rollback(ctx)` straight away. After a successful commit the rollback does nothing (Python: `with conn.transaction():`).
2. Insert the project with `RETURNING created_at`. Use `NULLIF($n, '')` for the hash and hint so an empty string is stored as NULL.
3. Insert each agent with `RETURNING created_at`.
4. `tx.Commit(ctx)`, then return the project with the timestamps filled in (call `.UTC()`).

### Translating duplicates

- `var pgErr *pgconn.PgError` and `errors.As(err, &pgErr)`, then `pgErr.Code == "23505"` (unique violation).
- Then check `pgErr.ConstraintName` to see which constraint fired. Do not match on message text. Read the real names with `\d projects` and `\d project_agents` in `make psql` (the agent primary key is probably `project_agents_pkey`), and confirm the unique index on `lower(name)` exists in `0001_projects.sql`.
- Project insert, name index fired: `ErrNameTaken`.
- Agent insert, agent primary key fired: `ErrAgentExists`.
- Any other unique violation: wrap and return it as a plain error.
- Wrap unexpected errors with context, for example `fmt.Errorf("creating project: %w", err)`. Never print the connection string.
- pgx v5 imports: `github.com/jackc/pgx/v5`, `.../pgxpool`, `.../pgconn`. Check `go.mod`.

### GetProject

1. Select the project with `COALESCE(key_hash, '')` and `COALESCE(key_hint, '')`, because pgx cannot scan NULL into a plain `string`.
2. `errors.Is(err, pgx.ErrNoRows)` becomes `ErrProjectNotFound`.
3. Select the agents `ORDER BY name`. Rows inserted in one transaction all get the same `created_at` (`now()` is the transaction start time), so ordering by it would be arbitrary.
4. `defer rows.Close()`, scan into `Agent` values, and check `rows.Err()` after the loop.

## 6. Progress

- [x] `project.go`, `store.go` (interface with `CreateProject`, five errors), `postgres.go` (struct, constructor, stub); `go build ./...` passes
- [ ] `newTestStore` helper compiles (`go vet`)
- [ ] `make test-db` target in the Makefile (plus `.PHONY` and `help`)
- [ ] Check constraint names and the `lower(name)` index (`\d projects`, `\d project_agents`)
- [ ] `newTestProject` helper with cleanup
- [ ] CreateProject tests 1 to 5, red against the stub
- [ ] CreateProject implemented, green
- [ ] Add `GetProject` to the interface, stub, tests 6 to 8, implement
- [ ] Remaining methods one at a time: list, delete, add / list / put / delete agent
- [ ] Handlers and the fake store in `internal/api`
- [ ] Postgres service container in CI for the store tests
