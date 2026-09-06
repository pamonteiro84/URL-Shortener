# URL Shortener

A small URL shortener written in Go. I'm building this mainly to learn Go properly — I come from a Rust/backend background, so a lot of the choices here are me figuring out what "idiomatic Go" actually looks like rather than porting over Rust habits.

## Stack

Go + Gin for the HTTP side, Postgres via GORM for storage, Redis for caching and rate limiting, and `gormigrate` for schema migrations instead of just letting GORM auto-migrate everything. Postgres and Redis run in Docker via Compose; the Go binary itself runs natively for now.

## How it's laid out

```
internal/
  router/      HTTP route -> handler wiring. Nothing else lives here.
  handlers/    HTTP in, service call, HTTP out. Parses requests, picks status codes.
               auth.go holds register/login/refresh/logout specifically.
  service/     the actual business logic (dedupe, turning storage errors into domain errors).
               auth_service.go holds the same for users/sessions.
  storage/     URLRepository and UserRepository interfaces. URLRepository has two implementations:
                 - gormURLRepository, the real one, talks to Postgres
                 - cachedURLRepository, a decorator that adds Redis in front of it
  shortcode/   pure function that turns a URL into a short code. No I/O.
  auth/        password hashing (Argon2id), JWT issuing/validation, refresh token generation.
               Pure crypto, doesn't know storage or HTTP exist.
  apperrors/   a small domain error type (Kind + the underlying error). Knows nothing about HTTP.
  models/      the URL and User structs.
  database/    Postgres connection setup.
  cache/       Redis connection setup.
  migrations/  one file per migration, gormigrate-style.
  middleware/  cross-cutting stuff that runs on every request that needs it:
                 - RateLimit, global, every route
                 - RequireAuth, only on the routes that need a logged-in user
```

The rule I've tried to stick to: each layer only talks to the one directly below it. Handlers never touch the DB, the service never sees a `gin.Context`, storage never makes decisions on its own (no code generation, no dedupe logic — it just stores and fetches what it's told to). `main.go` is the only place where everything gets wired together; it doesn't do anything on its own.

A `GET /:code` request, end to end: it hits the rate limit middleware first (checks the caller's IP against Redis), gets routed to the `Redirect` handler, which asks the service for the original URL, which asks storage — storage checks Redis first, and only falls back to Postgres (populating the cache on the way) if it's not there. If nothing's found anywhere, that turns into a 404 by the time it gets back to the handler.

A `POST /shorten` request additionally hits `RequireAuth` before the rate limit even matters much — it reads the `Authorization: Bearer` header, validates the JWT, and stashes the user ID on the request context for the handler to pick up. No auth, no shorten.

## Decisions worth knowing about

**Short codes are deterministic**, not random — I hash the URL (SHA-256, truncated to 6 bytes, base64 url-safe encoded) instead of generating something random. Same URL in, same code out, always. One side effect of this: if you shorten a URL that's already been shortened, you get the *same* code back (200) instead of a new one (201) — that's the dedupe check.

Real hash collisions (two different URLs landing on the same 6-byte code) aren't handled specially. The odds are astronomically low for anything this project will ever see, so if it ever happened it'd just fail on the DB's unique constraint.

Errors are split into two layers on purpose: `apperrors.AppError` carries a `Kind` (NotFound, AlreadyExists, ...) with zero knowledge of HTTP, and only the handlers layer (in one central function, `statusFor`) decides what HTTP status that maps to. That split is what let me add the Redis cache later without service or handlers needing to change — the cache lives entirely inside a `storage` decorator that implements the same interface as the real repository, so nothing above it even knows it's there.

Rate limiting is a simple fixed window — 10 requests/minute per IP, using plain `INCR` + `EXPIRE` in Redis rather than a Lua script. There's a tiny theoretical gap (a crash between those two commands could leave a key stuck without a TTL), but for something running on my own machine that's not worth the extra complexity of scripting right now.

**Auth is a hybrid of JWT and Redis.** A short-lived access token (JWT, ~15 min) covers normal requests — stateless, so `RequireAuth` never touches Redis or Postgres just to check who's asking. A long-lived opaque refresh token (~7 days) lives in Redis and is what `/refresh` and `/logout` actually check against. The reason for splitting it this way: a JWT alone can't be revoked before it expires without keeping a list of dead tokens somewhere, which just brings a database back into the picture anyway — so instead the JWT stays short and disposable, and the refresh token is the one thing that can be killed on logout. It rotates on every `/refresh` too: the old one stops working the moment a new one is issued.

Passwords are hashed with Argon2id (`golang.org/x/crypto/argon2`), the current OWASP recommendation, not bcrypt.

Once URLs got owners, the short-code hash had to change: it's now a hash of `userID + originalURL` rather than just the URL. Otherwise two different users shortening the same link would land on the same code, and only whichever of them got there first would actually own it — a confusing edge case that isn't worth having once ownership is real. One consequence worth knowing if you're reading the cache decorator: the Redis cache entry stores the owning `userID` alongside the URL now, not just the original URL string — the ownership check on delete goes through the same cache, so it needs the full picture on a hit, not just enough to redirect.

## Endpoints

| Method | Route | What it does |
|---|---|---|
| GET | `/health` | health check |
| POST | `/register` | create an account, `{"email": "...", "password": "..."}` |
| POST | `/login` | `{"email": "...", "password": "..."}` in, access token in the body + refresh token as an `HttpOnly` cookie |
| POST | `/refresh` | reads the refresh cookie, returns a new access token and rotates the cookie |
| POST | `/logout` | revokes the refresh token |
| POST | `/shorten` | **requires login.** shorten a URL, `{"url": "..."}` in, `{"short_code": "..."}` out |
| GET | `/:code` | redirects to the original URL, or 404 — public, no login needed |
| DELETE | `/:code` | **requires login, and requires owning the link.** 204, 403 if it's not yours, or 404 |

All of them go through rate limiting.

## Running it

You'll need Docker (or colima) and Go installed.

```bash
cp .env.example .env   # fill in POSTGRES_USER/PASSWORD/DB and REDIS_ADDR
make docker-up         # starts colima + brings up Postgres and Redis
make run
```

```bash
curl -X POST localhost:8080/register -H "Content-Type: application/json" -d '{"email":"me@example.com","password":"hunter2"}'

TOKEN=$(curl -c cookies.txt -X POST localhost:8080/login -H "Content-Type: application/json" \
  -d '{"email":"me@example.com","password":"hunter2"}' | jq -r .access_token)

curl -X POST localhost:8080/shorten -H "Content-Type: application/json" -H "Authorization: Bearer $TOKEN" \
  -d '{"url":"https://example.com"}'
# {"short_code":"EAaArVRs"}

curl -i localhost:8080/EAaArVRs   # 302, redirects to https://example.com — no auth needed

curl -X DELETE localhost:8080/EAaArVRs -H "Authorization: Bearer $TOKEN"   # 204
```

## Tests

`make test`, `make vet`, `make fmt`. The `storage` and `middleware` packages use `miniredis` to fake Redis in tests, so nothing needs real infrastructure running to test the caching, rate-limiting, or session logic. The `router` package has full HTTP integration tests via `httptest` instead of unit-testing handlers in isolation — felt more useful to test the whole request/response cycle at once, register-then-login-then-act included.

## What's deliberately not here

Listing all URLs and editing an existing one are both missing on purpose, not because I forgot them:

- **Listing** would mean building a `GET /urls` (or similar) scoped to the caller's own links — auth exists now to make that safe, it just isn't wired up yet.
- **Editing** the original URL would break the "short code is a deterministic hash of the URL" invariant that everything else relies on.

## Ideas for later

A graceful shutdown instead of just blocking forever on `r.Run`, actually dockerizing the Go app itself instead of only Postgres/Redis, some CI to run the test suite on push, and maybe swapping the rate limiter for a sliding window if the fixed-window boundary bursts ever actually become a problem in practice.
