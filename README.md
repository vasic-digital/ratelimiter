# digital.vasic.ratelimiter

A standalone Go module providing rate limiting with in-memory and Redis-backed
implementations, plus an HTTP middleware adapter. Standalone, project-agnostic,
fully reusable per **CONST-051(B)** (Submodules-As-Equal-Codebase + Decoupling
+ Dependency-Layout Mandate).

## Why this exists

Rate limiting is the load-bearing safety floor under every public API and
multi-tenant service. A wrong implementation does not just degrade — it
either over-throttles legitimate traffic (visible-to-user breakage) or
silently under-throttles abusive traffic (invisible-to-user breakage that
shows up as a billing surprise or a downstream outage). This module is
written, tested, and **proven by Challenge** to neither over- nor
under-throttle on its advertised configuration.

## Features

- **Sliding-window algorithm** — smoothed counting across N sub-windows
  avoids the burst-at-boundary failure mode of fixed windows.
- **In-memory limiter** (`pkg/memory`) — single-instance deployments,
  automatic idle-key cleanup, no external dependency.
- **Redis-backed limiter** (`pkg/redis`) — distributed, multi-instance,
  atomic Lua script. Tests use `miniredis` (no live Redis required for
  unit tests, but the Challenge wrapper can be extended to drive a real
  Redis container).
- **HTTP middleware** (`pkg/middleware`) — composes with any
  `net/http` router (standard library, chi, gorilla/mux, Gin). Emits
  `Retry-After` header (RFC 7231 §7.1.3) on every 429.
- **Fail-open by design** — middleware errors do not deny traffic;
  failure-mode is documented and tested.

## Anti-bluff guarantees (round-279)

Per the parent HelixCode constitution (Article XI §11.9, CONST-035,
CONST-048, CONST-050) every claim in this README is backed by a real
runtime exerciser:

> Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17): *"all
> existing tests and Challenges do work in anti-bluff manner — they
> MUST confirm that all tested codebase really works as expected! We
> had been in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does not
> work and can't be used! This MUST NOT be the case and execution of
> tests and Challenges MUST guarantee the quality, the completion and
> full usability by end users of the product!"*

1. `challenges/runner/main.go` instantiates the **real** limiter, wraps
   the **real** HTTP middleware around a **real** `httptest.Server`,
   and hits a **real** localhost socket — no test double of the system
   under test. It then asserts observed Allow/Deny counts and
   `Retry-After` header presence against expected values; mismatch
   fails the Challenge.
2. `challenges/ratelimiter_describe_challenge.sh` runs the runner in
   REAL mode (must exit 0) **and** in PAIRED-MUTATION mode
   (`RATELIMITER_MUTATE=1`) which bypasses the limiter entirely; the
   mutation variant MUST exit non-zero (translated to canonical exit
   99). A passing run proves the Challenge has *teeth* — it is not
   vacuously green.
3. `challenges/fixtures/locales.json` ships 5 bilingual rate-limit keys
   covering English, Serbian Cyrillic, Japanese, Spanish, and German.
   The limiter MUST be byte-faithful — Unicode-normalization-induced
   key collisions would fail the Challenge.
4. `docs/test-coverage.md` is a symbol→test ledger making the
   "every exported symbol has a runtime-evidence path" property
   mechanically inspectable.

## Installation

```bash
go get digital.vasic.ratelimiter
```

## Usage

### In-Memory Rate Limiter

```go
package main

import (
    "context"
    "fmt"
    "time"

    "digital.vasic.ratelimiter/pkg/limiter"
    "digital.vasic.ratelimiter/pkg/memory"
)

func main() {
    cfg := &limiter.Config{
        Rate:   100,
        Window: time.Minute,
        Burst:  0, // defaults to Rate via EffectiveBurst()
    }
    rl := memory.New(cfg)
    defer rl.Stop()

    result, err := rl.Allow(context.Background(), "user:123")
    if err != nil {
        panic(err)
    }
    if result.Allowed {
        fmt.Printf("Allowed. %d/%d remaining.\n", result.Remaining, result.Limit)
    } else {
        fmt.Printf("Rate limited. Retry after %s.\n", result.RetryAfter)
    }
}
```

### Redis-Backed Rate Limiter

```go
import (
    rlredis "digital.vasic.ratelimiter/pkg/redis"
    goredis "github.com/redis/go-redis/v9"
)

client := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
defer client.Close()

rl := rlredis.New(client, &limiter.Config{Rate: 1000, Window: time.Minute, Burst: 1200},
    rlredis.WithPrefix("myapp:"))

result, _ := rl.Allow(ctx, "api-key:abc")
```

### HTTP Middleware

```go
import (
    "net/http"
    "digital.vasic.ratelimiter/pkg/middleware"
)

mux := http.NewServeMux()
mux.HandleFunc("/api/data", handler)

rl := memory.New(&limiter.Config{Rate: 60, Window: time.Minute})
defer rl.Stop()

handler := middleware.HTTPMiddleware(rl, middleware.IPKeyFunc())(mux)
http.ListenAndServe(":8080", handler)
```

### Custom Key Functions + Limited Handler

```go
// Rate-limit by API key header (falls back to RemoteAddr if header absent).
handler := middleware.HTTPMiddleware(rl, middleware.HeaderKeyFunc("X-API-Key"))(mux)

opts := &middleware.Options{
    KeyFunc: middleware.IPKeyFunc(),
    OnLimited: func(w http.ResponseWriter, r *http.Request, result *limiter.Result) {
        w.Header().Set("Content-Type", "text/plain")
        w.WriteHeader(http.StatusTooManyRequests)
        fmt.Fprintf(w, "Slow down! Try again in %s", result.RetryAfter)
    },
}
handler := middleware.HTTPMiddlewareWithOptions(rl, opts)(mux)
```

## Package layout

| Package           | Description                                              |
|-------------------|----------------------------------------------------------|
| `pkg/limiter`     | Core interfaces (`Limiter`, `Config`, `Result`)          |
| `pkg/sliding`     | Sliding-window counter (granularity-configurable)        |
| `pkg/memory`      | In-memory limiter with background cleanup                |
| `pkg/redis`       | Redis-backed distributed limiter (atomic Lua)            |
| `pkg/middleware`  | HTTP middleware adapter, `KeyFunc` + `OnLimited` hooks   |
| `pkg/adaptive`    | Adaptive rate adjustment (load-aware)                    |
| `pkg/ladder`      | Tiered rate-limit ladders                                |
| `pkg/throttler`   | Throttler primitives                                     |
| `pkg/tokenbucket` | Token-bucket alternative                                 |
| `pkg/gin`         | Gin framework integration                                |

## Testing

```bash
# Unit tests (race-detected)
go test -race ./...

# Round-279 Challenge (real + paired-mutation)
bash challenges/ratelimiter_describe_challenge.sh
# expected output ends with: round-279 SUMMARY: real=PASS mutation=PROVED (exit 99)

# Real Challenge run only
bash challenges/ratelimiter_describe_challenge.sh real

# Paired-mutation only
bash challenges/ratelimiter_describe_challenge.sh mutate
```

Redis tests use [miniredis](https://github.com/alicebob/miniredis) and
require no running Redis instance for the unit suite. Drive a real
Redis container for full integration coverage.

## Definition of Done

A change to this module is NOT done until:

1. `go test -race ./...` passes — captured terminal output, same session.
2. `bash challenges/ratelimiter_describe_challenge.sh` reports
   `real=PASS mutation=PROVED (exit 99)` — captured terminal output,
   same session.
3. `docs/test-coverage.md` updated if the change adds/renames/removes
   an exported symbol.
4. The PR body contains a fenced `## Demo` block with the exact command
   output above.

## CONST-053 hygiene

`.gitignore` covers build artefacts, caches, secrets, runtime captures.
No tracked file matches any forbidden pattern; pre-commit audit per
gate `CM-GITIGNORE-PRECOMMIT-AUDIT` mandatory.

## CONST-061 force-push posture

This repo follows the parent constitution's pre-force-push merge-first
pipeline. Force-push to `main` requires explicit per-operation operator
approval AND the 4-step merge-first audit recorded in
`docs/changelogs/`.

## License

See LICENSE file.
