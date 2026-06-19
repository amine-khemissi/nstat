package dim

import (
	"testing"
	"time"
)

func TestDHCPLeaseScore(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	const lease = 86400 // 24h

	cases := []struct {
		name   string
		expiry int64
		avail  bool
		want   Score
	}{
		{"unavailable", 0, false, Good},             // no NM/lease info -> don't alarm
		{"healthy", now.Unix() + 40000, true, Good}, // ~11h left
		{"low", now.Unix() + 3600, true, Warn},      // 1h left, < lease/10 (2.4h)
		{"expired", now.Unix() - 10, true, Crit},    // past expiry
		{"avail-but-zero", 0, true, Good},           // avail but no expiry parsed
	}
	for _, c := range cases {
		if got := DHCPLeaseScore(c.expiry, lease, c.avail, now); got != c.want {
			t.Errorf("%s: score = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDHCPLeaseDisplay(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	const lease = 86400
	cases := []struct {
		name   string
		expiry int64
		avail  bool
		want   string
	}{
		{"unavailable", 0, false, "n/a"},
		{"expired", now.Unix() - 5, true, "EXPIRED"},
		{"low", now.Unix() + 3600, true, "renewing"},   // < lease/10
		{"healthy", now.Unix() + 40000, true, "valid"}, // ~11h
	}
	for _, c := range cases {
		if got := DHCPLeaseDisplay(c.expiry, lease, c.avail, now); got != c.want {
			t.Errorf("%s: display = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDHCPLeaseRemaining(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	if got := DHCPLeaseRemaining(0, false, now); got != "n/a" {
		t.Errorf("unavailable = %q, want n/a", got)
	}
	if got := DHCPLeaseRemaining(now.Unix()-5, true, now); got != "expired" {
		t.Errorf("expired = %q, want expired", got)
	}
	if got := DHCPLeaseRemaining(now.Unix()+3*3600+1800, true, now); got != "3h30m left" {
		t.Errorf("remaining = %q, want '3h30m left'", got)
	}
}
