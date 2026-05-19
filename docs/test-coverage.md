# RateLimiter — Test Coverage Ledger (round-279)

This ledger maps every exported symbol of `digital.vasic.ratelimiter` to the
tests (unit + challenge) that exercise it with real assertions. It satisfies
the parent HelixCode cascade — **CONST-035 (Zero-Bluff)**, **CONST-048
(Full-Automation-Coverage)**, **CONST-050 (No-Fakes-Beyond-Unit-Tests +
100% Test-Type Coverage)**, **Article XI §11.9** — by making the
"would-a-real-user-see-this-break" guarantee mechanically inspectable.

> Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17): *"all existing
> tests and Challenges do work in anti-bluff manner — they MUST confirm
> that all tested codebase really works as expected! We had been in
> position that all tests do execute with success and all Challenges as
> well, but in reality the most of the features does not work and can't
> be used! This MUST NOT be the case and execution of tests and
> Challenges MUST guarantee the quality, the completion and full
> usability by end users of the product!"*

## Symbol → Test ledger

### `pkg/limiter`

| Symbol                     | Kind      | Unit test(s)                               | Challenge                                                                                 |
|----------------------------|-----------|--------------------------------------------|-------------------------------------------------------------------------------------------|
| `Limiter` interface        | interface | `pkg/limiter/limiter_test.go` (compile)    | `challenges/runner/main.go` exercises via concrete impl                                   |
| `Config` struct            | struct    | `pkg/limiter/limiter_test.go`              | runner passes real `Config{Rate:3, Window:5s, Burst:3}`                                   |
| `Config.EffectiveBurst()`  | method    | `pkg/limiter/limiter_test.go`              | runner relies on Burst boundary — exit 1 if `EffectiveBurst()` regresses                  |
| `DefaultConfig()`          | func      | `pkg/limiter/limiter_test.go`              | runner does NOT rely on defaults — covered by unit only                                   |
| `Result` struct            | struct    | `pkg/memory/memory_test.go` field asserts  | runner inspects `Allowed` (via status) + `RetryAfter` (via header)                        |

### `pkg/memory`

| Symbol                | Kind  | Unit test(s)                                                                       | Challenge                                                                                          |
|-----------------------|-------|------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------------|
| `New(cfg)`            | func  | `pkg/memory/memory_test.go` + `memory_edge_test.go`                                | runner constructs real limiter at start of every run                                               |
| `RateLimiter.Allow`   | method| `memory_test.go` (within-limit, exceed, reset behaviours) + `memory_coverage_test.go` | runner sends 4 requests per locale, expects 3 allowed + 1 denied — proves boundary held end-to-end |
| `RateLimiter.Reset`   | method| `memory_test.go` (post-reset behaviour)                                            | not exercised by runner — unit-only coverage                                                       |
| `RateLimiter.Stop`    | method| `memory_test.go` (idempotency)                                                     | runner defers `Stop()` — leak check via go-race detector                                           |

### `pkg/sliding`

| Symbol              | Kind   | Unit test(s)                                              | Challenge                                                                          |
|---------------------|--------|-----------------------------------------------------------|------------------------------------------------------------------------------------|
| `NewWindow`         | func   | `pkg/sliding/sliding_test.go` + `sliding_coverage_test.go`| transitively exercised — `memory.New` calls it under the hood                       |
| `Window.Allow`      | method | `sliding_test.go`                                          | transitively exercised — runner allow/deny boundary is the system-level assertion   |
| `Window.Count`      | method | `sliding_coverage_test.go`                                 | not directly exercised — unit-only                                                  |

### `pkg/middleware`

| Symbol                          | Kind | Unit test(s)                  | Challenge                                                                                       |
|---------------------------------|------|-------------------------------|-------------------------------------------------------------------------------------------------|
| `HTTPMiddleware`                | func | `pkg/middleware/middleware_test.go` | runner wraps it around a real `httptest.Server`, hits real localhost socket, asserts status + headers |
| `HTTPMiddlewareWithOptions`     | func | `middleware_test.go`          | runner uses base `HTTPMiddleware` form; options-form covered by unit                            |
| `IPKeyFunc`                     | func | `middleware_test.go`          | runner uses `HeaderKeyFunc` instead — `IPKeyFunc` covered by unit                               |
| `HeaderKeyFunc`                 | func | `middleware_test.go`          | runner passes `X-Ratelimit-Key` per locale fixture — header path is exercised end-to-end        |
| `DefaultOnLimited`              | func | `middleware_test.go`          | runner asserts `Retry-After` integer-seconds header on every 429 — this function emits it      |
| `Options` struct                | struct | `middleware_test.go`        | runner uses defaults — alternate Options path covered by unit                                   |

## Test types per CONST-050(B)

| Type                       | Where                                                                          |
|----------------------------|--------------------------------------------------------------------------------|
| Unit                       | `pkg/*/_test.go` — mocks permitted per CONST-050(A)                            |
| Integration                | `challenges/runner/main.go` — real httptest.Server + real limiter wired together |
| E2E                        | wrapper drives full real-side flow including HTTP transit + header inspection  |
| Full automation            | `challenges/ratelimiter_describe_challenge.sh` real + paired-mutation modes    |
| Security (rate-limit DoS)  | runner's deny-path with 429 + Retry-After is the DoS-resilience floor          |
| UX / locale-bilingual      | 5-locale fixtures (en, sr, ja, es, de) — Cyrillic + CJK + Latin-extended       |
| Paired-mutation            | `RATELIMITER_MUTATE=1` variant MUST exit 99 — proves teeth                     |

## Anti-bluff guarantees

1. **No mocks in non-unit paths.** The Challenge runner instantiates real
   `memory.New(...)`, wraps the real `middleware.HTTPMiddleware`, and hits
   a real `httptest.Server`. No test double of the system under test.
2. **Positive runtime evidence per check.** Every request prints
   `[locale] label req=N status=S retry-after="X"`. The summary line
   prints observed vs expected counts before the assertion.
3. **Paired-mutation discrimination test.** The mutation variant bypasses
   the limiter entirely; the assertion catches it and the wrapper
   translates the failure to canonical exit 99 — proving the real run is
   not vacuously green.
4. **5-locale bilingual fixture coverage.** Cyrillic + CJK + extended
   Latin keys force the limiter to be byte-faithful — historical
   regression class where map-key collisions on normalized strings
   produced over-counting cannot pass undetected.
5. **Definition of Done.** A change to `pkg/memory`, `pkg/middleware`,
   `pkg/sliding`, or `pkg/limiter` is NOT done until
   `bash challenges/ratelimiter_describe_challenge.sh` reports
   `real=PASS mutation=PROVED (exit 99)` in the same session.

## Runbook

```bash
# Real Challenge run only (must exit 0)
bash challenges/ratelimiter_describe_challenge.sh real

# Paired-mutation only (must exit 99)
bash challenges/ratelimiter_describe_challenge.sh mutate

# Both (must report real=PASS mutation=PROVED, exit 0)
bash challenges/ratelimiter_describe_challenge.sh
```

For unit-test coverage:

```bash
go test -race ./...
```
