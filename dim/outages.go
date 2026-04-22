package dim

import (
	"fmt"
	"time"
)

// Outages tracks distinct connectivity outages (N consecutive ping losses).
type Outages struct {
	Count       int
	recentTimes []int64 // unix seconds
	InOutage    bool
	OutageStart int64
}

func NewOutages() *Outages { return &Outages{} }

func (o *Outages) RecordOutage(ts int64) {
	o.Count++
	o.InOutage = true
	o.OutageStart = ts
	o.recentTimes = append(o.recentTimes, ts)
	if len(o.recentTimes) > 20 {
		o.recentTimes = o.recentTimes[1:]
	}
}

func (o *Outages) RecordRecovery() {
	o.InOutage = false
	o.OutageStart = 0
}

func (o *Outages) CountRecent(window time.Duration) int {
	cutoff := time.Now().Unix() - int64(window.Seconds())
	n := 0
	for _, ts := range o.recentTimes {
		if ts >= cutoff {
			n++
		}
	}
	return n
}

func (o *Outages) RecentTimes() []int64 { return o.recentTimes }

func (o *Outages) Name() string           { return "Outages/1h" }
func (o *Outages) CSVFile() string        { return "" } // not logged to CSV
func (o *Outages) Unit() string           { return "count" }
func (o *Outages) Value() float64         { return float64(o.CountRecent(time.Hour)) }
func (o *Outages) IsOK() bool             { return true }
func (o *Outages) WarnThreshold() float64 { return 1 }
func (o *Outages) CritThreshold() float64 { return 3 }
func (o *Outages) Score() Score           { return ScoreOf(o.Value(), true, 1, 3) }
func (o *Outages) DisplayValue() string {
	return fmt.Sprintf("%d  (%d total)", o.CountRecent(time.Hour), o.Count)
}
