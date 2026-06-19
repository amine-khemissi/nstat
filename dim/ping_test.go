package dim

import "testing"

func TestPacketLossWindowed(t *testing.T) {
	_, s := NewPacketLoss(10)
	// 2 drops out of 10 pings -> 20% over the window.
	pat := []bool{false, true, false, false, true, false, false, false, false, false}
	for _, lost := range pat {
		if lost {
			s.OnPingFailure()
		} else {
			s.OnPingSuccess(10)
		}
	}
	if s.Total != 10 || s.Lost != 2 {
		t.Fatalf("Total/Lost = %d/%d, want 10/2", s.Total, s.Lost)
	}
	if s.LossPct != 20 {
		t.Errorf("LossPct = %.1f, want 20", s.LossPct)
	}
}

func TestPacketLossAgesOut(t *testing.T) {
	_, s := NewPacketLoss(10)
	s.OnPingFailure() // one early drop
	for i := 0; i < 10; i++ {
		s.OnPingSuccess(10) // fill the window with successes
	}
	if s.LossPct != 0 {
		t.Errorf("LossPct = %.2f, want 0 after the drop aged out", s.LossPct)
	}
	if s.Total != 10 {
		t.Errorf("Total = %d, want 10 (window cap)", s.Total)
	}
}

func TestPacketLossDefaultWindow(t *testing.T) {
	_, s := NewPacketLoss(0) // invalid -> default
	s.OnPingSuccess(10)
	if cap(s.window) != defaultLossWindow {
		t.Errorf("window cap = %d, want %d", cap(s.window), defaultLossWindow)
	}
}
