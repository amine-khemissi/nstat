package dim

import (
	"fmt"
	"math"
)

// PingStats holds the rolling window shared by the RTT and Jitter dimensions.
// It implements PingObserver; RTT and Jitter read from it via pointer.
type PingStats struct {
	window []float64
	size   int
	Avg    float64
	Jitter float64
}

func NewPingStats(windowSize int) *PingStats {
	return &PingStats{size: windowSize}
}

func (ps *PingStats) OnPingSuccess(rttMs float64) {
	ps.window = append(ps.window, rttMs)
	if len(ps.window) > ps.size {
		ps.window = ps.window[1:]
	}
	ps.Avg, ps.Jitter = rttStats(ps.window)
}

func (ps *PingStats) OnPingFailure() {}

func rttStats(w []float64) (avg, jitter float64) {
	if len(w) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range w {
		sum += v
	}
	avg = sum / float64(len(w))
	if len(w) == 1 {
		return avg, 0
	}
	variance := 0.0
	for _, v := range w {
		d := v - avg
		variance += d * d
	}
	return avg, math.Sqrt(variance / float64(len(w)))
}

// --- RTT dimension ----------------------------------------------------------

type RTT struct{ ps *PingStats }

func NewRTT(ps *PingStats) *RTT { return &RTT{ps: ps} }

func (r *RTT) Name() string           { return "RTT (avg)" }
func (r *RTT) CSVFile() string        { return "csv_rtt_avg.csv" }
func (r *RTT) Unit() string           { return "ms" }
func (r *RTT) Value() float64         { return r.ps.Avg }
func (r *RTT) IsOK() bool             { return true }
func (r *RTT) WarnThreshold() float64 { return 80 }
func (r *RTT) CritThreshold() float64 { return 200 }
func (r *RTT) Score() Score           { return ScoreOf(r.ps.Avg, true, 80, 200) }
func (r *RTT) DisplayValue() string   { return FmtMs(r.ps.Avg) }

// --- Jitter dimension -------------------------------------------------------

type Jitter struct{ ps *PingStats }

func NewJitter(ps *PingStats) *Jitter { return &Jitter{ps: ps} }

func (j *Jitter) Name() string           { return "Jitter" }
func (j *Jitter) CSVFile() string        { return "csv_jitter.csv" }
func (j *Jitter) Unit() string           { return "ms" }
func (j *Jitter) Value() float64         { return j.ps.Jitter }
func (j *Jitter) IsOK() bool             { return true }
func (j *Jitter) WarnThreshold() float64 { return 10 }
func (j *Jitter) CritThreshold() float64 { return 30 }
func (j *Jitter) Score() Score           { return ScoreOf(j.ps.Jitter, true, 10, 30) }
func (j *Jitter) DisplayValue() string   { return FmtMs(j.ps.Jitter) }

// --- PacketLoss dimension ---------------------------------------------------

const defaultLossWindow = 60

// LossStats measures ICMP packet loss over a sliding window of recent pings, so
// the figure reflects current conditions and recovers as old drops age out
// (rather than being a lifetime cumulative average).
type LossStats struct {
	window []bool // ring buffer; true == lost
	pos    int
	filled bool
	size   int

	Total   int     // pings in the window
	Lost    int     // dropped pings in the window
	LossPct float64 // windowed loss %
}

type PacketLoss struct{ s *LossStats }

func NewPacketLoss(windowSize int) (*PacketLoss, *LossStats) {
	if windowSize <= 0 {
		windowSize = defaultLossWindow
	}
	s := &LossStats{size: windowSize}
	return &PacketLoss{s: s}, s
}

func (s *LossStats) OnPingSuccess(_ float64) { s.record(false) }
func (s *LossStats) OnPingFailure()          { s.record(true) }

func (s *LossStats) record(lost bool) {
	if len(s.window) == 0 {
		s.window = make([]bool, s.size)
	}
	s.window[s.pos] = lost
	s.pos = (s.pos + 1) % s.size
	if s.pos == 0 {
		s.filled = true
	}
	n := s.pos
	if s.filled {
		n = s.size
	}
	lostCount := 0
	for i := 0; i < n; i++ {
		if s.window[i] {
			lostCount++
		}
	}
	s.Total, s.Lost = n, lostCount
	if n > 0 {
		s.LossPct = float64(lostCount) / float64(n) * 100
	} else {
		s.LossPct = 0
	}
}

func (pl *PacketLoss) Name() string           { return "Packet loss" }
func (pl *PacketLoss) CSVFile() string        { return "csv_packet_loss.csv" }
func (pl *PacketLoss) Unit() string           { return "%" }
func (pl *PacketLoss) Value() float64         { return pl.s.LossPct }
func (pl *PacketLoss) IsOK() bool             { return true }
func (pl *PacketLoss) WarnThreshold() float64 { return 1 }
func (pl *PacketLoss) CritThreshold() float64 { return 5 }
func (pl *PacketLoss) Score() Score           { return LossScore(pl.s.LossPct, pl.s.Total) }
func (pl *PacketLoss) DisplayValue() string {
	return fmt.Sprintf("%.1f%%  (%d/%d)", pl.s.LossPct, pl.s.Lost, pl.s.Total)
}
