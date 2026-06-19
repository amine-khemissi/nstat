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
	if got := DHCPLeaseDisplay(0, false, now); got != "n/a" {
		t.Errorf("unavailable display = %q, want n/a", got)
	}
	if got := DHCPLeaseDisplay(now.Unix()-5, true, now); got != "EXPIRED" {
		t.Errorf("expired display = %q, want EXPIRED", got)
	}
	if got := DHCPLeaseDisplay(now.Unix()+3*3600+1800, true, now); got != "3h30m left" {
		t.Errorf("display = %q, want '3h30m left'", got)
	}
}
