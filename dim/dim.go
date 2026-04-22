// Package dim defines the Dimension interface and the observer interfaces used
// by the daemon to dispatch measurement results. To add a new dimension:
//  1. Create a new file in this package.
//  2. Implement Dimension (for status/graph/csv display).
//  3. Implement whichever observer interface matches the probe type
//     (PingObserver, TCPObserver, DNSObserver, DHCPObserver).
//  4. Register the new struct in daemon/daemon.go.
package dim

import "fmt"

// Score represents a health level for a dimension.
type Score int

const (
	Good Score = iota
	Warn
	Crit
)

func (s Score) String() string {
	switch s {
	case Good:
		return "GOOD"
	case Warn:
		return "WARN"
	case Crit:
		return "CRIT"
	default:
		return "?"
	}
}

// Dimension is the read-only view of a single network metric used for
// status display, CSV logging, and graph generation.
type Dimension interface {
	Name() string
	CSVFile() string
	Unit() string
	Value() float64
	IsOK() bool
	WarnThreshold() float64
	CritThreshold() float64
	Score() Score
	DisplayValue() string
}

// --- observer interfaces ----------------------------------------------------

// PingObserver is implemented by dimensions that react to each ICMP probe.
type PingObserver interface {
	OnPingSuccess(rttMs float64)
	OnPingFailure()
}

// TCPObserver is implemented by dimensions that react to TCP connect probes.
type TCPObserver interface {
	OnTCPResult(ok bool, ms float64)
}

// DNSObserver is implemented by dimensions that react to DNS resolve probes.
type DNSObserver interface {
	OnDNSResult(ok bool, ms float64)
}

// DHCPObserver is implemented by dimensions that react to DHCP server pings.
type DHCPObserver interface {
	OnDHCPResult(ok bool, ms float64)
}

// --- shared helpers ---------------------------------------------------------

func ScoreOf(value float64, ok bool, warn, crit float64) Score {
	if !ok {
		return Crit
	}
	if value >= crit {
		return Crit
	}
	if value >= warn {
		return Warn
	}
	return Good
}

func FmtMs(v float64) string  { return fmt.Sprintf("%.1f ms", v) }
func FmtPct(v float64) string { return fmt.Sprintf("%.1f%%", v) }
