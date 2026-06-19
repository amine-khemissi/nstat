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

// Sliding-window sizing for TCP loss. TCP targets are probed on the LAN
// interval, so a bounded window reflects the recent past rather than the whole
// daemon session — loss recovers as old failures age out instead of being a
// permanent lifetime average.
const (
	defaultTCPWindow = 50
	minLossSamples   = 25 // hold at Good below this many samples (avoids spikes)
)

// LossScore scores a windowed loss percentage (TCP or ICMP), holding at Good
// until the window has enough samples so a single early failure in a small
// window can't spike to CRIT. Shared by the TCP-loss and packet-loss dimensions.
func LossScore(lossPct float64, total int) Score {
	if total < minLossSamples {
		return Good
	}
	return ScoreOf(lossPct, true, 1, 5)
}

// TCPStats holds the shared state for the TCPConnect and TCPLoss dimensions.
//
// Loss is measured over a sliding window of the most recent attempts and counts
// only timeouts — genuine unreachability. A refused/reset/DNS failure means the
// packet made a round trip (the path works), so it is kept in the breakdown but
// excluded from the loss percentage.
type TCPStats struct {
	LastMs     float64
	LastOK     bool
	LastReason TCPFailReason

	// ring buffer of the most recent outcomes (TCPFailNone == success)
	window []TCPFailReason
	pos    int
	filled bool

	// derived from the current window
	Total   int     // attempts in the window
	Fail    int     // all failures in the window (any non-None)
	LossPct float64 // timeout-only loss across the window

	// failure breakdown over the window
	TimeoutCount int
	RefusedCount int
	ResetCount   int
	OtherCount   int
}

func (s *TCPStats) OnTCPResult(ok bool, ms float64) {
	reason := TCPFailNone
	if !ok {
		reason = TCPFailOther
	}
	s.OnTCPResultWithReason(ok, ms, reason)
}

func (s *TCPStats) OnTCPResultWithReason(ok bool, ms float64, reason TCPFailReason) {
	if ok {
		reason = TCPFailNone
		s.LastMs = ms
		s.LastOK = true
	} else {
		s.LastMs = 0
		s.LastOK = false
	}
	s.LastReason = reason

	if len(s.window) == 0 {
		s.window = make([]TCPFailReason, defaultTCPWindow)
	}
	s.window[s.pos] = reason
	s.pos = (s.pos + 1) % len(s.window)
	if s.pos == 0 {
		s.filled = true
	}
	s.recompute()
}

// recompute derives the windowed totals and breakdown from the ring buffer.
func (s *TCPStats) recompute() {
	n := s.pos
	if s.filled {
		n = len(s.window)
	}
	s.Total, s.Fail = n, 0
	s.TimeoutCount, s.RefusedCount, s.ResetCount, s.OtherCount = 0, 0, 0, 0
	timeouts := 0
	for i := 0; i < n; i++ {
		switch s.window[i] {
		case TCPFailNone:
			// success
		case TCPFailTimeout:
			s.Fail++
			s.TimeoutCount++
			timeouts++
		case TCPFailRefused:
			s.Fail++
			s.RefusedCount++
		case TCPFailReset:
			s.Fail++
			s.ResetCount++
		default: // TCPFailDNS, TCPFailOther
			s.Fail++
			s.OtherCount++
		}
	}
	if n > 0 {
		s.LossPct = float64(timeouts) / float64(n) * 100
	} else {
		s.LossPct = 0
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
func (t *TCPLoss) Score() Score           { return LossScore(t.s.LossPct, t.s.Total) }
func (t *TCPLoss) DisplayValue() string {
	// loss% is timeout-only; the count shown is timeouts/attempts in the window.
	return fmt.Sprintf("%.1f%%  (%d/%d)", t.s.LossPct, t.s.TimeoutCount, t.s.Total)
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

func NewTCPMulti(targets []struct {
	Host string
	Port int
}) *TCPMulti {
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
