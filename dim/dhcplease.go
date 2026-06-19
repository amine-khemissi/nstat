package dim

import (
	"fmt"
	"time"
)

// DHCPLease reports the health of the current DHCP lease (server identifier and
// time remaining), read from the system's DHCP client rather than probed — an
// active DHCP exchange needs to bind privileged port 68, which the unprivileged
// daemon can't do. "Remaining" is the time until the lease expires; a healthy
// client renews around the halfway mark, so a small remaining value means
// renewal has been failing.
type DHCPLease struct {
	server    string
	expiry    int64 // epoch seconds when the lease expires (0 == unknown)
	leaseTime int64 // total lease duration in seconds
	avail     bool  // lease info could be read at all
	now       func() time.Time
}

func NewDHCPLease() *DHCPLease {
	return &DHCPLease{now: time.Now}
}

// OnLease records the latest lease reading.
func (d *DHCPLease) OnLease(server string, expiry, leaseTime int64, avail bool) {
	d.server, d.expiry, d.leaseTime, d.avail = server, expiry, leaseTime, avail
}

func (d *DHCPLease) Server() string { return d.server }

// remainingSec returns seconds until the lease expires (may be negative).
func (d *DHCPLease) remainingSec() int64 {
	return d.expiry - d.now().Unix()
}

func (d *DHCPLease) Name() string {
	if d.server != "" {
		return fmt.Sprintf("DHCP %s", d.server)
	}
	return "DHCP lease"
}
func (d *DHCPLease) CSVFile() string { return "csv_dhcp_lease.csv" }
func (d *DHCPLease) Unit() string    { return "h" }

// Value is the hours of lease remaining (for graphing); 0 when unknown/expired.
func (d *DHCPLease) Value() float64 {
	if !d.avail || d.expiry == 0 {
		return 0
	}
	r := d.remainingSec()
	if r < 0 {
		return 0
	}
	return float64(r) / 3600
}

func (d *DHCPLease) IsOK() bool { return !d.avail || d.remainingSec() > 0 }

// WarnThreshold/CritThreshold are expressed in the same unit as Value (hours).
func (d *DHCPLease) WarnThreshold() float64 { return float64(d.leaseTime) / 10 / 3600 }
func (d *DHCPLease) CritThreshold() float64 { return 0 }

func (d *DHCPLease) Score() Score { return DHCPLeaseScore(d.expiry, d.leaseTime, d.avail, d.now()) }

func (d *DHCPLease) DisplayValue() string { return DHCPLeaseDisplay(d.expiry, d.avail, d.now()) }

// DHCPLeaseScore scores a lease by time remaining. Shared with `nstat status`
// (which renders from the persisted snapshot, not the live dimension).
func DHCPLeaseScore(expiry, leaseTime int64, avail bool, now time.Time) Score {
	if !avail || expiry == 0 {
		return Good // no lease info available (e.g. no NetworkManager) — don't alarm
	}
	rem := expiry - now.Unix()
	switch {
	case rem <= 0:
		return Crit // lease expired — renewal failed
	case leaseTime > 0 && rem < leaseTime/10:
		return Warn // running low; client should have renewed by now
	default:
		return Good
	}
}

// DHCPLeaseDisplay renders the lease state for the status table.
func DHCPLeaseDisplay(expiry int64, avail bool, now time.Time) string {
	if !avail || expiry == 0 {
		return "n/a"
	}
	rem := time.Duration(expiry-now.Unix()) * time.Second
	if rem <= 0 {
		return "EXPIRED"
	}
	return fmt.Sprintf("%s left", fmtLeaseDur(rem))
}

func fmtLeaseDur(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
