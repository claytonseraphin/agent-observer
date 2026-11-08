# Authentication and access: design

Brainstormed 2026-11-05. Companion to `claude/backend-todo.md` (Phase 3 for the seam, Phase 6c for the build) and `claude/projects-api-schema.md`. Nothing here is built yet. No code until the phase it belongs to.

## 1. Two planes, two mechanisms

| Plane | Who | How they authenticate | Status |
|---|---|---|---|
| Data plane | agents and the Python / npm packages sending traffic through the proxy | project key in `X-Observer-Key` (keyless traffic goes to `default`) | designed, Phase 4 |
| Management plane | people using the portal and `/api/*` | admin token, then GitHub login, then roles | this document |

Logging in does **not** replace project keys. A person logs in to create a project and obtain its key; the agents use the key.

## 2. Deployment model

- **Self-hosted first.** A team pulls the image and runs `docker compose up`. Their copy, their database, their network. Nothing that happens there is visible to us. Any usage reporting would be a separate opt-in feature, not designed.
- A hosted version (we run it, others sign up) is possible later. It would need mandatory login, strict isolation, TLS, rate limits and private-address blocking. It is on the Later list.
- **The database is still required.** It is the running copy's own memory: the agent registry (name to upstream URL), projects, keys, and later users and sessions. The developers' own application databases are never touched.
- **Database choices:** Postgres everywhere. Either the bundled compose service or **bring your own** by setting `DATABASE_URL` (already works). A bring-your-own server should get its own database or schema, not the application's. Other engines (MySQL and so on) are not supported: the migrations and error handling use Postgres features (`timestamptz`, `lower(name)` unique index, `ON CONFLICT`, `23505`).
- SQLite is a possible later "try it" mode behind the `ProjectStore` interface. It would be labelled single-machine, not the production path (one file on one disk, no high availability, awkward on container platforms).

## 3. Why not "knowing the URL is enough"

Without protection, anyone who can reach the URL can:
- **Read** agent names, upstream URLs, timings and error messages (internal topology, even with the no-raw-prompts default).
- **Write**: create and delete projects, and register agents with any upstream URL. The proxy forwards to that URL, so it can be made to call internal addresses (SSRF). This is the most serious one.

| Where it runs | Risk |
|---|---|
| localhost / a laptop | negligible |
| private network or VPN, one trusted team | low (the network is the boundary) |
| reachable from a wider network or the internet | real; needs protection |

Defaults that follow from this: compose publishes the proxy port on `127.0.0.1` only; startup logs a clear warning when `AUTH_MODE=none` and the server listens on a non-local address.

## 4. Decisions

| Decision | Choice |
|---|---|
| Modes | `AUTH_MODE=none` (default) / `token` / `github`. More providers later. |
| `none` | No checks. Everything runs as one built-in user. For laptops and local development. |
| `token` | The management API needs one shared secret, `ADMIN_TOKEN`, compared in constant time. For small teams on a private network. |
| `github` | GitHub login, sessions, owners, roles, invites. The token stays as a break-glass admin. |
| Where the token lives | In `.env`, never in git. Passed to the container by compose and read **when the container starts**, not at image build, so the image holds no secret and the same image runs everywhere. |
| Token strength | Long and random (for example `openssl rand -hex 32`). Refuse to start in `token` mode if it is empty or short. |
| First owner (bootstrap) | The deployer sets `ADMIN_TOKEN`, enters it on the portal, creates the first project and invites the team. Nobody needs to sign up first. |
| Owner links GitHub | Optional but recommended. After the token login the owner clicks "Connect GitHub"; the identity is attached to the **same user record**, so existing projects stay theirs. Later the deployer may remove the token or keep it for recovery. Needs the GitHub OAuth app configured; otherwise the team stays in token mode. |
| Identity key | `(provider, subject)`, never email. Emails change, and merging accounts on an unverified email is an account-takeover route. |
| Login provider | GitHub only for now. GitHub is OAuth2, not OIDC: after login call the API for the user and primary verified email. Libraries: `golang.org/x/oauth2` (and `coreos/go-oidc` when OIDC arrives). |
| Authorization | Owned by this tool, not by the login provider. The provider only proves who the person is. An optional rule `GITHUB_ALLOWED_ORG` can restrict login to one organization. |
| Roles (per project) | `viewer` (read), `editor` (read and write; this is "both"), `owner` (editor, plus delete, invite and manage members). Stored in `project_members(project_id, user_id, role)`. |
| Invites | The owner enters an email and a role. The tool creates a one-time invite link (random token stored hashed, with an expiry). Sending email is optional (SMTP settings); **copying the link must always work**. The invite attaches to whichever GitHub identity opens the link, not to an email match. |
| Sessions | Server-side, stored in Postgres (token hash, user, expiry), in an HttpOnly, Secure, SameSite=Lax cookie. Survive restarts. Revocable. Not JWTs. |
| CSRF | Cookie sessions need it: check the `Origin` header on state-changing requests, plus SameSite=Lax. |
| Handler seam | Handlers read the current user from `r.Context()` and ask one function, `Can(user, project, action)`. Today it answers from mode and owner, later from `project_members`. Handlers never know which mode is active. |
| Not-allowed response | Another user's project returns **404**, not 403, so existence is not leaked. |
| `default` project | System-owned, visible to every logged-in user. Revisit for a hosted version. |
| Unknown / unmatched | Public routes are only login, callback and health. `/api/groups` (open and unscoped today) must be covered by the same middleware before anything is exposed. |

## 5. Flows

### Token login (bootstrap)
1. Deployer puts `ADMIN_TOKEN` in `.env` and runs `docker compose up`.
2. Portal asks for the token. The proxy compares it with its environment value (constant time). On a match it creates a session for the built-in bootstrap user.
3. Restarts do not ask again: the token is configuration, and sessions live in Postgres until they expire. If the token is lost, set a new one in `.env` and restart; projects are untouched.

### What survives what

| Event | Token | Sessions | Projects and keys |
|---|---|---|---|
| Container restart or image upgrade | stays | stay | stay |
| `docker compose down` | stays | stay | stay (named volume) |
| `docker compose down -v` or `make reset` | stays | gone | **gone** (volume deleted) |
| Token changed in `.env` | new value | stay | stay |

### GitHub login
Authorization-code flow with PKCE and a `state` check. After the callback: fetch the user, find or create the `identities(provider, subject)` row, apply the optional org rule (needs the `read:org` scope; without it only public memberships are visible), create a session.

### Linking (owner) and invites (everyone else)
Linking attaches a GitHub identity to the existing bootstrap user. An invitee opens the link, signs in with GitHub, and the invite attaches the role to that identity.

## 6. Data model (migration `0002` and later)

```
users(id, email, name, created_at)
identities(provider, subject, user_id references users, primary key (provider, subject))
sessions(token_hash primary key, user_id references users, expires_at, created_at)
projects.owner_id  -- nullable, references users
-- later:
project_members(project_id references projects on delete cascade, user_id references users, role, primary key (project_id, user_id))
invites(token_hash primary key, project_id, role, created_by, expires_at, used_at)
```

## 7. Build order

| Step | What | Handlers need |
|---|---|---|
| A (with Phase 3) | middleware seam: `User` type, context helper, `Can(...)`, `AUTH_MODE=none` | built against it from the start |
| B | `token` mode: login page, sessions table, `users` and nullable `owner_id`, bootstrap user | no change |
| C | GitHub login, owner linking, optional org rule, `identities` | no change |
| D | members, roles, invites; `Can` reads `project_members`; listing returns only projects the user may see | a role check per handler (already behind `Can`) |
| Later | Google Workspace / generic OIDC (domain via the `hd` claim, covers Okta and Entra), personal Gmail by invite only, SQLite mode, hosted version | none |

Personal `@gmail.com` accounts have no organization to check, so they only work through explicit invites. Google Workspace can be grouped by domain, which is the "corporate email" path.

## 8. Security checklist (for steps B to D)

- Token compared in constant time; rate-limit attempts; never log tokens, keys or invite links.
- OAuth: validate `state`, use PKCE, only redirect to known paths after login (no open redirect), register exact callback URLs.
- Cookies: HttpOnly, Secure (except local development), SameSite=Lax, short lifetime. Re-check org membership at session refresh or keep sessions short, because removing someone from the org does not end an existing session.
- Org rule needs `read:org`, and an org owner may have to approve the OAuth app; a GitHub App avoids that but is more setup. Allow a configurable API base for GitHub Enterprise Server.
- Invite and session tokens stored hashed, with expiries; invites single use.
- Block private and metadata addresses for upstream URLs before any shared deployment (already in Phase 3).
- Secrets (client ID and secret, token) only in `.env` or the platform's secret store.

## 9. Open questions

- Is the portal served by the proxy on the same origin, or a separate dev server? Same origin keeps cookies simple; a separate origin needs CORS with credentials. **Not answered yet; assume same origin.**
- Session lifetime (suggested 8 to 24 hours) and whether to re-check org membership on refresh.
- Role names (`viewer` / `editor` / `owner`) and whether a tool-level admin is needed beyond project owners.
- Whether the keyless `default` fallback should stay on once auth is enabled.
- Optional SMTP for invite emails: which settings, and is copy-the-link enough for v1.
