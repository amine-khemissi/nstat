package dim

import "fmt"

// MTUProbe tracks the path MTU discovered by the daemon's Don't-Fragment (DF)
// probe. DetectedSize is the largest ICMP packet (total IPv4 size, in bytes)
// that reaches the target without being fragmented — i.e. the real path MTU.
type MTUProbe struct {
	DetectedSize int     // last measured path MTU (total IPv4 bytes)
	LastMs       float64 // latency at the detected size
	LastOK       bool    // false when the last probe could not measure the MTU
	Total        int
	Fail         int
}

func NewMTUProbe() *MTUProbe {
	return &MTUProbe{LastOK: true}
}

// OnMTUDetected records a successful path-MTU measurement. Unlike a sticky
// high-water mark, this always reflects the most recent probe so the reported
// value tracks the live path (e.g. drops to 1492 on a PPPoE link).
func (m *MTUProbe) OnMTUDetected(mtu int, ms float64) {
	m.Total++
	m.DetectedSize = mtu
	m.LastMs = ms
	m.LastOK = true
}

// OnMTUError records a probe that could not determine the MTU (socket error,
// ICMP filtered, or host unreachable).
func (m *MTUProbe) OnMTUError() {
	m.Total++
	m.Fail++
	m.LastOK = false
}

func (m *MTUProbe) Name() string    { return "MTU probe" }
func (m *MTUProbe) CSVFile() string { return "csv_mtu.csv" }
func (m *MTUProbe) Unit() string    { return "bytes" }
func (m *MTUProbe) Value() float64  { return float64(m.DetectedSize) }
func (m *MTUProbe) IsOK() bool      { return m.LastOK && m.DetectedSize >= 1400 }

func (m *MTUProbe) WarnThreshold() float64 { return 1400 } // below typical ethernet/PPPoE
func (m *MTUProbe) CritThreshold() float64 { return 1200 } // severely limited

func (m *MTUProbe) Score() Score {
	if !m.LastOK {
		return Warn
	}
	if m.DetectedSize < 1200 {
		return Crit
	}
	if m.DetectedSize < 1400 {
		return Warn
	}
	return Good
}

func (m *MTUProbe) DisplayValue() string {
	if m.Total == 0 {
		return "not tested"
	}
	if !m.LastOK {
		return "probe failed"
	}
	return fmt.Sprintf("%d bytes", m.DetectedSize)
}

// DetectedMTU returns the last measured path MTU.
func (m *MTUProbe) DetectedMTU() int { return m.DetectedSize }

// HasIssue reports whether the last probe failed or the path MTU is abnormally
// low. A normal sub-1500 MTU (e.g. 1492 on PPPoE) is *not* an issue.
func (m *MTUProbe) HasIssue() bool { return !m.LastOK || m.DetectedSize < 1400 }
