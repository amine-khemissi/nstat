package dim

import "fmt"

// TCPFailReason categorizes why a TCP connection failed.
type TCPFailReason int

const (
	TCPFailNone    TCPFailReason = iota // no failure
	TCPFailTimeout                      // connection timed out (no response)
	TCPFailRefused                      // connection refused (RST received)
	TCPFailReset                        // connection reset during handshake
	TCPFailDNS                          // DNS resolution failed
	TCPFailOther                        // other error
)

func (r TCPFailReason) String() string {
	switch r {
	case TCPFailNone:
		return "ok"
	case TCPFailTimeout:
		return "timeout"
	case TCPFailRefused:
		return "refused"
	case TCPFailReset:
		return "reset"
	case TCPFailDNS:
		return "dns"
	default:
		return "other"
	}
}

// TCPStats holds the shared state for the TCPConnect and TCPLoss dimensions.
type TCPStats struct {
	LastMs     float64
	LastOK     bool
	LastReason TCPFailReason
	Total      int
	Fail       int
	LossPct    float64

	// Failure breakdown
	TimeoutCount int
	RefusedCount int
	ResetCount   int
	OtherCount   int
}

func (s *TCPStats) OnTCPResult(ok bool, ms float64) {
	s.OnTCPResultWithReason(ok, ms, TCPFailNone)
}

func (s *TCPStats) OnTCPResultWithReason(ok bool, ms float64, reason TCPFailReason) {
	s.Total++
	s.LastReason = reason
	if ok {
		s.LastMs = ms
		s.LastOK = true
	} else {
		s.LastMs = 0
		s.LastOK = false
		s.Fail++
		switch reason {
		case TCPFailTimeout:
			s.TimeoutCount++
		case TCPFailRefused:
			s.RefusedCount++
		case TCPFailReset:
			s.ResetCount++
		default:
			s.OtherCount++
		}
	}
	if s.Total > 0 {
		s.LossPct = float64(s.Fail) / float64(s.Total) * 100
	}
}

// --- TCPConnect dimension ---------------------------------------------------

type TCPConnect struct {
	s    *TCPStats
	host string
	port int
}

func NewTCPConnect(host string, port int) (*TCPConnect, *TCPStats) {
	s := &TCPStats{LastOK: true}
	return &TCPConnect{s: s, host: host, port: port}, s
}

func (t *TCPConnect) Name() string           { return fmt.Sprintf("TCP %s:%d", t.host, t.port) }
func (t *TCPConnect) CSVFile() string        { return "csv_tcp_connect.csv" }
func (t *TCPConnect) Unit() string           { return "ms" }
func (t *TCPConnect) Value() float64         { return t.s.LastMs }
func (t *TCPConnect) IsOK() bool             { return t.s.LastOK }
func (t *TCPConnect) WarnThreshold() float64 { return 150 }
func (t *TCPConnect) CritThreshold() float64 { return 150 }
func (t *TCPConnect) Score() Score           { return ScoreOf(t.s.LastMs, t.s.LastOK, 150, 150) }
func (t *TCPConnect) DisplayValue() string   { return FmtMs(t.s.LastMs) }

// --- TCPLoss dimension ------------------------------------------------------

type TCPLoss struct{ s *TCPStats }

func NewTCPLoss(s *TCPStats) *TCPLoss { return &TCPLoss{s: s} }

func (t *TCPLoss) Name() string           { return "TCP loss" }
func (t *TCPLoss) CSVFile() string        { return "csv_tcp_loss.csv" }
func (t *TCPLoss) Unit() string           { return "%" }
func (t *TCPLoss) Value() float64         { return t.s.LossPct }
func (t *TCPLoss) IsOK() bool             { return true }
func (t *TCPLoss) WarnThreshold() float64 { return 1 }
func (t *TCPLoss) CritThreshold() float64 { return 5 }
func (t *TCPLoss) Score() Score           { return ScoreOf(t.s.LossPct, true, 1, 5) }
func (t *TCPLoss) DisplayValue() string {
	return fmt.Sprintf("%.1f%%  (%d/%d)", t.s.LossPct, t.s.Fail, t.s.Total)
}

// FailureBreakdown returns a string showing the breakdown of failure types.
func (t *TCPLoss) FailureBreakdown() string {
	if t.s.Fail == 0 {
		return ""
	}
	return fmt.Sprintf("timeout:%d refused:%d reset:%d other:%d",
		t.s.TimeoutCount, t.s.RefusedCount, t.s.ResetCount, t.s.OtherCount)
}

// --- Multi-target TCP tracking ----------------------------------------------

type TCPTarget struct {
	Host  string
	Port  int
	Stats *TCPStats
}

type TCPMulti struct {
	Targets []*TCPTarget
}

func NewTCPMulti(targets []struct{ Host string; Port int }) *TCPMulti {
	m := &TCPMulti{}
	for _, t := range targets {
		m.Targets = append(m.Targets, &TCPTarget{
			Host:  t.Host,
			Port:  t.Port,
			Stats: &TCPStats{LastOK: true},
		})
	}
	return m
}

func (m *TCPMulti) RecordResult(host string, port int, ok bool, ms float64, reason TCPFailReason) {
	for _, t := range m.Targets {
		if t.Host == host && t.Port == port {
			t.Stats.OnTCPResultWithReason(ok, ms, reason)
			return
		}
	}
}

func (m *TCPMulti) GetTarget(host string, port int) *TCPTarget {
	for _, t := range m.Targets {
		if t.Host == host && t.Port == port {
			return t
		}
	}
	return nil
}

// OverallLossPct returns the average loss across all targets.
func (m *TCPMulti) OverallLossPct() float64 {
	if len(m.Targets) == 0 {
		return 0
	}
	var total, fail int
	for _, t := range m.Targets {
		total += t.Stats.Total
		fail += t.Stats.Fail
	}
	if total == 0 {
		return 0
	}
	return float64(fail) / float64(total) * 100
}
