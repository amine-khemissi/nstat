package dim

import "testing"

// feed pushes a sequence of outcomes into a fresh TCPStats.
func feed(reasons ...TCPFailReason) *TCPStats {
	s := &TCPStats{LastOK: true}
	for _, r := range reasons {
		s.OnTCPResultWithReason(r == TCPFailNone, 10, r)
	}
	return s
}

func TestLossIsTimeoutOnly(t *testing.T) {
	// 1 timeout + 1 refused + 1 reset out of 10 attempts: only the timeout is loss.
	s := feed(
		TCPFailNone, TCPFailNone, TCPFailTimeout, TCPFailNone, TCPFailRefused,
		TCPFailNone, TCPFailReset, TCPFailNone, TCPFailNone, TCPFailNone,
	)
	if s.Total != 10 {
		t.Fatalf("Total = %d, want 10", s.Total)
	}
	if s.Fail != 3 {
		t.Errorf("Fail = %d, want 3 (all failures)", s.Fail)
	}
	if got, want := s.LossPct, 10.0; got != want {
		t.Errorf("LossPct = %.1f, want %.1f (timeout-only)", got, want)
	}
	if s.TimeoutCount != 1 || s.RefusedCount != 1 || s.ResetCount != 1 {
		t.Errorf("breakdown = to:%d ref:%d rst:%d, want 1/1/1",
			s.TimeoutCount, s.RefusedCount, s.ResetCount)
	}
}

func TestBreakdownKeepsRealReason(t *testing.T) {
	// Regression: failures fed via the plain OnTCPResult path must not all land
	// in "other" — but the daemon now uses OnTCPResultWithReason, so verify the
	// reason is honored end to end.
	s := feed(TCPFailTimeout, TCPFailRefused)
	if s.OtherCount != 0 {
		t.Errorf("OtherCount = %d, want 0 (reasons must be classified)", s.OtherCount)
	}
}

func TestWindowAgesOutFailures(t *testing.T) {
	s := &TCPStats{LastOK: true}
	// One timeout, then fill the whole window with successes.
	s.OnTCPResultWithReason(false, 0, TCPFailTimeout)
	for i := 0; i < defaultTCPWindow; i++ {
		s.OnTCPResultWithReason(true, 10, TCPFailNone)
	}
	if s.LossPct != 0 {
		t.Errorf("LossPct = %.2f, want 0 after the timeout aged out of the window", s.LossPct)
	}
	if s.Total != defaultTCPWindow {
		t.Errorf("Total = %d, want %d (window cap)", s.Total, defaultTCPWindow)
	}
}

func TestMinSampleFloor(t *testing.T) {
	// A single timeout in a tiny window is a high percentage, but below the
	// minimum sample count it must not score worse than Good.
	if got := LossScore(100, 1); got != Good {
		t.Errorf("score(100%%, 1 sample) = %v, want Good", got)
	}
	if got := LossScore(0, minLossSamples); got != Good {
		t.Errorf("score(0%%, %d) = %v, want Good", minLossSamples, got)
	}
	if got := LossScore(6, minLossSamples); got != Crit {
		t.Errorf("score(6%%, %d) = %v, want Crit", minLossSamples, got)
	}
	if got := LossScore(2, minLossSamples); got != Warn {
		t.Errorf("score(2%%, %d) = %v, want Warn", minLossSamples, got)
	}
}
