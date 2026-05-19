// Package main provides the round-279 Challenge runner for digital.vasic.ratelimiter.
//
// This binary is a REAL exerciser of the RateLimiter API. It satisfies the
// anti-bluff invariants cascaded from the parent HelixCode constitution
// (CONST-035, CONST-048, CONST-050, Article XI §11.9):
//
//  1. It does NOT mock the system under test. It instantiates the actual
//     in-memory sliding-window limiter from pkg/memory and the actual HTTP
//     middleware from pkg/middleware against an httptest.Server hitting a real
//     localhost socket.
//  2. It captures positive runtime evidence — observed Allow/Deny counts,
//     HTTP status codes (200 vs 429), and Retry-After header values — and
//     asserts each one against an expected value. Mismatch => non-zero exit.
//  3. It loads 5-locale bilingual fixtures (en, sr, ja, es, de) to confirm
//     the limiter is locale-agnostic when the key derives from non-ASCII
//     input — historically a class of bugs where map-key collisions on
//     normalized strings produced silent over-counting.
//  4. The companion bash wrapper
//     challenges/ratelimiter_describe_challenge.sh runs this binary, then
//     runs a paired-mutation variant (RATELIMITER_MUTATE=1) which forces the
//     limiter to allow EVERY request — that variant MUST exit 99, proving
//     this challenge has teeth.
//
// Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17):
//
//	"all existing tests and Challenges do work in anti-bluff manner - they
//	MUST confirm that all tested codebase really works as expected! We had
//	been in position that all tests do execute with success and all
//	Challenges as well, but in reality the most of the features does not
//	work and can't be used! This MUST NOT be the case and execution of
//	tests and Challenges MUST guarantee the quality, the completion and
//	full usability by end users of the product!"
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"digital.vasic.ratelimiter/pkg/limiter"
	"digital.vasic.ratelimiter/pkg/memory"
	"digital.vasic.ratelimiter/pkg/middleware"
)

const (
	exitOK             = 0
	exitChallengeFail  = 1
	exitMutationProved = 99 // paired-mutation expected exit
)

type fixture struct {
	Locale string `json:"locale"`
	Key    string `json:"key"`
	// Label is bilingual: English + native-script. Both must round-trip
	// through the limiter without producing distinct sliding windows when
	// keys are byte-identical, and MUST produce distinct windows when keys
	// differ — i.e. the limiter is byte-faithful, not unicode-folding.
	Label string `json:"label"`
}

// fixturesDir resolves the fixtures path relative to this source file so the
// runner works from any working directory. CONST-053 forbids hardcoded
// distribution hosts but fixture paths are intra-repo and exempt.
func fixturesDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "fixtures"), nil
}

func loadFixtures() ([]fixture, error) {
	dir, err := fixturesDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "locales.json")
	b, err := os.ReadFile(path) // #nosec G304 — intra-repo fixture path
	if err != nil {
		return nil, fmt.Errorf("read fixtures: %w", err)
	}
	var out []fixture
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode fixtures: %w", err)
	}
	if len(out) < 5 {
		return nil, fmt.Errorf("expected >=5 locale fixtures, got %d", len(out))
	}
	return out, nil
}

// runChallenge exercises the limiter against a real HTTP server and asserts
// the observed Allow/Deny pattern matches the configured Rate/Burst.
func runChallenge(mutate bool) (int, error) {
	fxs, err := loadFixtures()
	if err != nil {
		return 0, err
	}

	cfg := &limiter.Config{
		Rate:   3,
		Window: 5 * time.Second,
		Burst:  3,
	}
	rl := memory.New(cfg)
	defer rl.Stop()

	var srv *httptest.Server
	if mutate {
		// PAIRED-MUTATION mode: bypass the limiter entirely. Every request
		// returns 200. This variant MUST cause the assertion below to fail
		// (observed deny != expected deny) and the binary exits 1, which
		// the wrapper translates to exit 99 — proving the real challenge
		// would catch a broken limiter.
		srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
	} else {
		// REAL mode: the actual HTTP middleware wraps a no-op handler.
		mw := middleware.HTTPMiddleware(rl, middleware.HeaderKeyFunc("X-Ratelimit-Key"))
		srv = httptest.NewServer(mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})))
	}
	defer srv.Close()

	totalAllow := 0
	totalDeny := 0

	for _, fx := range fxs {
		// Per-key budget: Burst=3, so first 3 requests allowed, 4th denied.
		for i := 0; i < 4; i++ {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
			if err != nil {
				return 0, fmt.Errorf("build request: %w", err)
			}
			req.Header.Set("X-Ratelimit-Key", fx.Key)
			resp, err := srv.Client().Do(req)
			if err != nil {
				return 0, fmt.Errorf("http call: %w", err)
			}
			ra := resp.Header.Get("Retry-After")
			status := resp.StatusCode
			_ = resp.Body.Close()

			switch status {
			case http.StatusOK:
				totalAllow++
			case http.StatusTooManyRequests:
				totalDeny++
				if ra == "" {
					return 0, fmt.Errorf("%s: 429 without Retry-After header", fx.Locale)
				}
				if _, err := strconv.Atoi(ra); err != nil {
					return 0, fmt.Errorf("%s: Retry-After %q not integer-seconds: %w", fx.Locale, ra, err)
				}
			default:
				return 0, fmt.Errorf("%s: unexpected status %d", fx.Locale, status)
			}
			fmt.Printf("  [%s] %-20s req=%d status=%d retry-after=%q\n", fx.Locale, truncate(fx.Label, 18), i+1, status, ra)
		}
	}

	// Expected: 5 locales x 3 allowed = 15 allows, 5 locales x 1 denied = 5 denies.
	expectAllow := len(fxs) * 3
	expectDeny := len(fxs) * 1
	fmt.Printf("\nObserved: allow=%d deny=%d (expect allow=%d deny=%d)\n", totalAllow, totalDeny, expectAllow, expectDeny)

	if totalAllow != expectAllow || totalDeny != expectDeny {
		return exitChallengeFail, fmt.Errorf("ASSERTION FAILED: allow/deny counts mismatch")
	}
	return exitOK, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Trim by rune boundary.
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

func main() {
	mutate := strings.EqualFold(os.Getenv("RATELIMITER_MUTATE"), "1") ||
		strings.EqualFold(os.Getenv("RATELIMITER_MUTATE"), "true")

	mode := "real"
	if mutate {
		mode = "MUTATION (expected to fail)"
	}
	fmt.Printf("=== digital.vasic.ratelimiter round-279 Challenge runner — mode=%s ===\n", mode)

	code, err := runChallenge(mutate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(exitChallengeFail)
	}
	if code != exitOK {
		os.Exit(code)
	}
	fmt.Println("\nPASS — all locales hit Rate-Limit boundary as expected, Retry-After present on every 429.")
	os.Exit(exitOK)
}
