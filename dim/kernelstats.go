package dim

import "fmt"

// KernelTCPStats tracks TCP statistics from /proc/net/snmp (Linux only).
type KernelTCPStats struct {
	// Current values
	RetransSegs   int64 // segments retransmitted
	OutSegs       int64 // total segments sent
	InSegs        int64 // total segments received
	InErrs        int64 // segments received with errors
	OutRsts       int64 // RST segments sent
	AttemptFails  int64 // failed connection attempts
	EstabResets   int64 // connections reset while established
	CurrEstab     int64 // currently established connections

	// Previous values (for calculating deltas)
	prevRetransSegs  int64
	prevOutSegs      int64
	prevInErrs       int64
	prevOutRsts      int64
	prevAttemptFails int64
	prevEstabResets  int64

	// Deltas (changes since last check)
	DeltaRetrans     int64
	DeltaOutSegs     int64
	DeltaInErrs      int64
	DeltaOutRsts     int64
	DeltaAttemptFails int64
	DeltaEstabResets int64

	// Calculated metrics
	RetransPct float64 // retransmit percentage

	initialized bool
}

func NewKernelTCPStats() *KernelTCPStats {
	return &KernelTCPStats{}
}

func (k *KernelTCPStats) Update(retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab int64) {
	if k.initialized {
		// Calculate deltas
		k.DeltaRetrans = retrans - k.prevRetransSegs
		k.DeltaOutSegs = outSegs - k.prevOutSegs
		k.DeltaInErrs = inErrs - k.prevInErrs
		k.DeltaOutRsts = outRsts - k.prevOutRsts
		k.DeltaAttemptFails = attemptFails - k.prevAttemptFails
		k.DeltaEstabResets = estabResets - k.prevEstabResets

		// Calculate retransmit percentage
		if k.DeltaOutSegs > 0 {
			k.RetransPct = float64(k.DeltaRetrans) / float64(k.DeltaOutSegs) * 100
		} else {
			k.RetransPct = 0
		}
	}

	// Store current as previous
	k.prevRetransSegs = retrans
	k.prevOutSegs = outSegs
	k.prevInErrs = inErrs
	k.prevOutRsts = outRsts
	k.prevAttemptFails = attemptFails
	k.prevEstabResets = estabResets

	// Update current values
	k.RetransSegs = retrans
	k.OutSegs = outSegs
	k.InSegs = inSegs
	k.InErrs = inErrs
	k.OutRsts = outRsts
	k.AttemptFails = attemptFails
	k.EstabResets = estabResets
	k.CurrEstab = currEstab

	k.initialized = true
}

// --- Dimension interface for TCP Retransmits --------------------------------

type TCPRetransmits struct {
	k *KernelTCPStats
}

func NewTCPRetransmits(k *KernelTCPStats) *TCPRetransmits {
	return &TCPRetransmits{k: k}
}

func (t *TCPRetransmits) Name() string           { return "TCP retrans" }
func (t *TCPRetransmits) CSVFile() string        { return "csv_tcp_retrans.csv" }
func (t *TCPRetransmits) Unit() string           { return "%" }
func (t *TCPRetransmits) Value() float64         { return t.k.RetransPct }
func (t *TCPRetransmits) IsOK() bool             { return t.k.RetransPct < 5 }
func (t *TCPRetransmits) WarnThreshold() float64 { return 2 }
func (t *TCPRetransmits) CritThreshold() float64 { return 5 }
func (t *TCPRetransmits) Score() Score           { return ScoreOf(t.k.RetransPct, true, 2, 5) }

func (t *TCPRetransmits) DisplayValue() string {
	if !t.k.initialized {
		return "collecting..."
	}
	return fmt.Sprintf("%.2f%%  (+%d/%d segs)", t.k.RetransPct, t.k.DeltaRetrans, t.k.DeltaOutSegs)
}

// --- Dimension interface for TCP Errors -------------------------------------

type TCPErrors struct {
	k *KernelTCPStats
}

func NewTCPErrors(k *KernelTCPStats) *TCPErrors {
	return &TCPErrors{k: k}
}

func (t *TCPErrors) Name() string    { return "TCP errors" }
func (t *TCPErrors) CSVFile() string { return "csv_tcp_errors.csv" }
func (t *TCPErrors) Unit() string    { return "count" }
func (t *TCPErrors) Value() float64  { return float64(t.k.DeltaInErrs + t.k.DeltaEstabResets) }
func (t *TCPErrors) IsOK() bool      { return t.k.DeltaInErrs == 0 && t.k.DeltaEstabResets == 0 }
func (t *TCPErrors) WarnThreshold() float64 { return 1 }
func (t *TCPErrors) CritThreshold() float64 { return 10 }

func (t *TCPErrors) Score() Score {
	total := t.k.DeltaInErrs + t.k.DeltaEstabResets
	return ScoreOf(float64(total), true, 1, 10)
}

func (t *TCPErrors) DisplayValue() string {
	if !t.k.initialized {
		return "collecting..."
	}
	return fmt.Sprintf("errs:%d rsts:%d fails:%d",
		t.k.DeltaInErrs, t.k.DeltaEstabResets, t.k.DeltaAttemptFails)
}

// Summary returns a detailed breakdown of kernel TCP stats.
func (k *KernelTCPStats) Summary() string {
	if !k.initialized {
		return "not yet initialized"
	}
	return fmt.Sprintf(
		"retrans=%.2f%% (+%d) inErrs=%d outRsts=%d attemptFails=%d estabResets=%d currEstab=%d",
		k.RetransPct, k.DeltaRetrans, k.DeltaInErrs, k.DeltaOutRsts,
		k.DeltaAttemptFails, k.DeltaEstabResets, k.CurrEstab,
	)
}
