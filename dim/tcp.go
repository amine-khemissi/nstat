package dim

import "fmt"

// TCPStats holds the shared state for the TCPConnect and TCPLoss dimensions.
type TCPStats struct {
	LastMs  float64
	LastOK  bool
	Total   int
	Fail    int
	LossPct float64
}

func (s *TCPStats) OnTCPResult(ok bool, ms float64) {
	s.Total++
	if ok {
		s.LastMs = ms
		s.LastOK = true
	} else {
		s.LastMs = 0
		s.LastOK = false
		s.Fail++
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
