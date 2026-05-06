package ladder_test

import (
	"testing"
	"time"

	"digital.vasic.ratelimiter/pkg/ladder"
)

func TestLadder_FirstFailure_ReturnsFirstStep(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second, 5 * time.Second})
	duration := l.RecordFailure("ip-a", time.Unix(1000, 0))
	if duration != 2*time.Second {
		t.Fatalf("expected 2s, got %s", duration)
	}
}

func TestLadder_SecondFailure_AdvancesToSecondStep(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second, 5 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	duration := l.RecordFailure("ip-a", time.Unix(1010, 0))
	if duration != 5*time.Second {
		t.Fatalf("expected 5s, got %s", duration)
	}
}

func TestLadder_BeyondLastStep_ClampsToLastStep(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second, 5 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	l.RecordFailure("ip-a", time.Unix(1010, 0))
	duration := l.RecordFailure("ip-a", time.Unix(1020, 0))
	if duration != 5*time.Second {
		t.Fatalf("expected 5s clamp, got %s", duration)
	}
}

func TestLadder_Reset_ClearsCounter(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second, 5 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	l.RecordFailure("ip-a", time.Unix(1010, 0))
	l.Reset("ip-a")
	duration := l.RecordFailure("ip-a", time.Unix(1020, 0))
	if duration != 2*time.Second {
		t.Fatalf("expected 2s after reset, got %s", duration)
	}
}

func TestLadder_CheckBlocked_BeforeFailure_NotBlocked(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second})
	blocked, _ := l.CheckBlocked("ip-a", time.Unix(1000, 0))
	if blocked {
		t.Fatal("expected not blocked")
	}
}

func TestLadder_CheckBlocked_AfterFailure_BlockedForDuration(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	blocked, retryAfter := l.CheckBlocked("ip-a", time.Unix(1001, 0))
	if !blocked {
		t.Fatal("expected blocked")
	}
	if retryAfter != 1*time.Second {
		t.Fatalf("expected 1s retry-after, got %s", retryAfter)
	}
}

func TestLadder_CheckBlocked_AfterExpiry_NotBlocked(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	blocked, _ := l.CheckBlocked("ip-a", time.Unix(1003, 0))
	if blocked {
		t.Fatal("expected not blocked after expiry")
	}
}

func TestLadder_New_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for empty steps")
		}
	}()
	ladder.New(nil)
}

func TestLadder_PerKeyIndependence(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second, 5 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	duration := l.RecordFailure("ip-b", time.Unix(1000, 0))
	if duration != 2*time.Second {
		t.Fatalf("ip-b should be at step 0; got %s", duration)
	}
}

func TestLadder_Prune_RemovesExpiredEntries(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	l.RecordFailure("ip-b", time.Unix(2000, 0))
	pruned := l.Prune(time.Unix(3000, 0), 100*time.Millisecond)
	if pruned != 2 {
		t.Fatalf("expected 2 pruned, got %d", pruned)
	}
}

func TestLadder_Prune_KeepsActiveEntries(t *testing.T) {
	l := ladder.New([]time.Duration{2 * time.Second})
	l.RecordFailure("ip-a", time.Unix(1000, 0))
	pruned := l.Prune(time.Unix(1001, 0), 1*time.Hour)
	if pruned != 0 {
		t.Fatalf("expected 0 pruned, got %d", pruned)
	}
}
