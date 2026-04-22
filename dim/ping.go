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

type LossStats struct {
	Total   int
	Lost    int
	LossPct float64
}

type PacketLoss struct{ s *LossStats }

func NewPacketLoss() (*PacketLoss, *LossStats) {
	s := &LossStats{}
	return &PacketLoss{s: s}, s
}

func (s *LossStats) OnPingSuccess(_ float64) {
	s.Total++
	s.updatePct()
}

func (s *LossStats) OnPingFailure() {
	s.Total++
	s.Lost++
	s.updatePct()
}

func (s *LossStats) updatePct() {
	if s.Total > 0 {
		s.LossPct = float64(s.Lost) / float64(s.Total) * 100
	}
}

func (pl *PacketLoss) Name() string           { return "Packet loss" }
func (pl *PacketLoss) CSVFile() string        { return "csv_packet_loss.csv" }
func (pl *PacketLoss) Unit() string           { return "%" }
func (pl *PacketLoss) Value() float64         { return pl.s.LossPct }
func (pl *PacketLoss) IsOK() bool             { return true }
func (pl *PacketLoss) WarnThreshold() float64 { return 1 }
func (pl *PacketLoss) CritThreshold() float64 { return 5 }
func (pl *PacketLoss) Score() Score           { return ScoreOf(pl.s.LossPct, true, 1, 5) }
func (pl *PacketLoss) DisplayValue() string {
	return fmt.Sprintf("%.1f%%  (%d/%d)", pl.s.LossPct, pl.s.Lost, pl.s.Total)
}
