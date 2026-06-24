# AGENTS.md

## INHERITED FROM constitution/AGENTS.md

All rules in `constitution/AGENTS.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `AGENTS.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

Instructions for AI agents working on this codebase.

## Module Structure

This is a pure Go library module (`digital.vasic.ratelimiter`). It has no `main` package and produces no binary. All public API is under `pkg/`.

## Key Interfaces

The `limiter.Limiter` interface is the central contract. Any new rate limiter backend must implement:
- `Allow(ctx context.Context, key string) (*Result, error)` - Check and record a request.
- `Reset(ctx context.Context, key string) error` - Clear rate limit state for a key.

## Adding a New Backend

1. Create a new package under `pkg/` (e.g., `pkg/memcached/`).
2. Implement the `limiter.Limiter` interface.
3. Add tests in the same package.
4. Ensure all tests pass: `go test ./... -count=1`.

## Testing

- In-memory tests use the `sliding` package directly.
- Redis tests use `alicebob/miniredis/v2` for a real Redis protocol without a running server.
- Middleware tests use `httptest` for HTTP handler testing.
- All tests must pass with `-count=1` (no caching).

## Dependencies

Keep dependencies minimal. Current external dependencies:
- `github.com/redis/go-redis/v9` - Redis client (only in `pkg/redis/`)
- `github.com/stretchr/testify` - Test assertions (test only)
- `github.com/alicebob/miniredis/v2` - In-process Redis (test only)
