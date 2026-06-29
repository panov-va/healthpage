package notify

import (
	"testing"
	"time"
)

func TestRetryBackoff(t *testing.T) {
	cases := []struct {
		attempt   int
		wantDelay time.Duration
		wantOK    bool
	}{
		{0, 0, false}, // некорректно
		{1, 1 * time.Minute, true},
		{2, 5 * time.Minute, true},
		{3, 30 * time.Minute, true},
		{4, 0, false}, // исчерпано → DLQ
		{99, 0, false},
	}
	for _, c := range cases {
		delay, ok := RetryBackoff(c.attempt)
		if ok != c.wantOK || delay != c.wantDelay {
			t.Errorf("RetryBackoff(%d) = (%v, %v), want (%v, %v)",
				c.attempt, delay, ok, c.wantDelay, c.wantOK)
		}
	}
	if MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", MaxAttempts)
	}
}
