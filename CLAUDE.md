# CLAUDE.md

## INHERITED FROM constitution/CLAUDE.md

All rules in `constitution/CLAUDE.md` (and the `constitution/Constitution.md` it references) apply unconditionally. This file's rules below extend them — they MUST NOT weaken any inherited rule. See parent root `CLAUDE.md` §6.AD for the Lava-specific incorporation context (29th §6.L cycle, 2026-05-14) and §6.AD-debt for the implementation-gap inventory. Use `constitution/find_constitution.sh` from the parent project root to resolve the absolute path of the submodule from any nested location.

## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution's `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution

This file provides guidance to Claude Code when working with this repository.

## Overview

`digital.vasic.ratelimiter` is a standalone Go module providing rate limiting implementations. It offers an in-memory sliding window limiter for single-instance deployments and a Redis-backed distributed limiter for multi-instance environments. An HTTP middleware adapter is included for easy integration with any `net/http` compatible router.

## Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./... -count=1

# Run tests with verbose output
go test -v ./... -count=1

# Run tests for a specific package
go test -v ./pkg/memory/ -count=1

# Run a single test
go test -v -run TestAllowWithinLimit ./pkg/memory/
```

## Architecture

```
pkg/limiter/     - Core interfaces (Limiter, Config, Result)
pkg/sliding/     - Sliding window counter algorithm
pkg/memory/      - In-memory rate limiter (uses sliding window)
pkg/redis/       - Redis-backed distributed rate limiter (Lua script)
pkg/middleware/   - HTTP middleware adapter
```

**Data flow:** HTTP Middleware -> Limiter interface -> Memory or Redis implementation -> Sliding window algorithm

**Key design decisions:**
- The `Limiter` interface is the central abstraction; all implementations satisfy it.
- The sliding window algorithm divides time into sub-windows for smooth rate limiting.
- The Redis implementation uses an atomic Lua script to avoid race conditions.
- The HTTP middleware is fail-open: on limiter errors, requests are allowed through.

## Conventions

- Functional options pattern for Redis limiter configuration (`WithPrefix`).
- Table-driven tests throughout.
- `*_test.go` files beside source files.
- `context.Context` passed through all interface methods.
- `EffectiveBurst()` centralizes the burst-defaults-to-rate logic.
