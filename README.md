# URL Shortener

A small URL shortener written in Go. I'm building this mainly to learn Go properly — I come from a Rust/backend background, so a lot of the choices here are me figuring out what "idiomatic Go" actually looks like rather than porting over Rust habits.

## Stack

Go + Gin for the HTTP side, Postgres via GORM for storage, Redis for caching and rate limiting, and `gormigrate` for schema migrations instead of just letting GORM auto-migrate everything. Postgres and Redis run in Docker via Compose; the Go binary itself runs natively for now.

## How it's laid out

```
internal/
  router/      HTTP route -> handler wiring. Nothing else lives here.
  handlers/    HTTP in, service call, HTTP out. Parses requests, picks status codes.
  service/     the actual business logic (dedupe, turning storage errors into domain errors).
  storage/     URLRepository interface, with two implementations:
                 - gormURLRepository, the real one, talks to Postgres
                 - cachedURLRepository, a decorator that adds Redis in front of it
  shortcode/   pure function that turns a URL into a short code. No I/O.
  apperrors/   a small domain error type (Kind + the underlying error). Knows nothing about HTTP.
  models/      just the URL struct.
  database/    Postgres connection setup.
  cache/       Redis connection setup.
  migrations/  one file per migration, gormigrate-style.
  middleware/  cross-cutting stuff that runs on every request (currently just rate limiting).
```

The rule I've tried to stick to: each layer only talks to the one directly below it. Handlers never touch the DB, the service never sees a `gin.Context`, storage never makes decisions on its own (no code generation, no dedupe logic — it just stores and fetches what it's told to). `main.go` is the only place where everything gets wired together; it doesn't do anything on its own.

A `GET /:code` request, end to end: it hits the rate limit middleware first (checks the caller's IP against Redis), gets routed to the `Redirect` handler, which asks the service for the original URL, which asks storage — storage checks Redis first, and only falls back to Postgres (populating the cache on the way) if it's not there. If nothing's found anywhere, that turns into a 404 by the time it gets back to the handler.

## Decisions worth knowing about

**Short codes are deterministic**, not random — I hash the URL (SHA-256, truncated to 6 bytes, base64 url-safe encoded) instead of generating something random. Same URL in, same code out, always. One side effect of this: if you shorten a URL that's already been shortened, you get the *same* code back (200) instead of a new one (201) — that's the dedupe check.

Real hash collisions (two different URLs landing on the same 6-byte code) aren't handled specially. The odds are astronomically low for anything this project will ever see, so if it ever happened it'd just fail on the DB's unique constraint.

Errors are split into two layers on purpose: `apperrors.AppError` carries a `Kind` (NotFound, AlreadyExists, ...) with zero knowledge of HTTP, and only the handlers layer (in one central function, `statusFor`) decides what HTTP status that maps to. That split is what let me add the Redis cache later without service or handlers needing to change — the cache lives entirely inside a `storage` decorator that implements the same interface as the real repository, so nothing above it even knows it's there.

Rate limiting is a simple fixed window — 10 requests/minute per IP, using plain `INCR` + `EXPIRE` in Redis rather than a Lua script. There's a tiny theoretical gap (a crash between those two commands could leave a key stuck without a TTL), but for something running on my own machine that's not worth the extra complexity of scripting right now.

## Endpoints

| Method | Route | What it does |
|---|---|---|
| GET | `/health` | health check |
| POST | `/shorten` | shorten a URL, `{"url": "..."}` in, `{"short_code": "..."}` out |
| GET | `/:code` | redirects to the original URL, or 404 |
| DELETE | `/:code` | deletes it, 204 or 404 |

All of them go through rate limiting.

## Running it

You'll need Docker (or colima) and Go installed.

```bash
cp .env.example .env   # fill in POSTGRES_USER/PASSWORD/DB and REDIS_ADDR
make docker-up         # starts colima + brings up Postgres and Redis
make run
```

```bash
curl -X POST localhost:8080/shorten -H "Content-Type: application/json" -d '{"url":"https://example.com"}'
# {"short_code":"EAaArVRs"}

curl -i localhost:8080/EAaArVRs   # 302, redirects to https://example.com

curl -X DELETE localhost:8080/EAaArVRs   # 204
```

## Tests

`make test`, `make vet`, `make fmt`. The `storage` and `middleware` packages use `miniredis` to fake Redis in tests, so nothing needs real infrastructure running to test the caching or rate-limiting logic. The `router` package has full HTTP integration tests via `httptest` instead of unit-testing handlers in isolation — felt more useful to test the whole request/response cycle at once.

## What's deliberately not here

Listing all URLs, editing an existing one, and auth are all missing on purpose, not because I forgot them:

- **Listing** would mean exposing every URL in the DB to anyone, since there's no concept of "who owns this link" yet.
- **Editing** the original URL would break the "short code is a deterministic hash of the URL" invariant that everything else relies on.
- **Auth** is really the prerequisite for the first two to make sense at all — but it's a big enough topic (users, password hashing, sessions/JWT) that it deserves to be its own thing rather than a quick add-on here.

## Ideas for later

Auth (see above), a graceful shutdown instead of just blocking forever on `r.Run`, actually dockerizing the Go app itself instead of only Postgres/Redis, some CI to run the test suite on push, and maybe swapping the rate limiter for a sliding window if the fixed-window boundary bursts ever actually become a problem in practice.
