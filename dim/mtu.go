package dim

import "fmt"

// MTUProbe tracks MTU/fragmentation issues by testing different packet sizes.
type MTUProbe struct {
	LastSize    int     // last successfully tested size
	MaxSize     int     // maximum size that worked
	LastOK      bool
	LastMs      float64
	Total       int
	Fail        int
	FailedSizes []int   // sizes that failed
}

func NewMTUProbe() *MTUProbe {
	return &MTUProbe{
		MaxSize:     1500, // assume standard MTU initially
		LastOK:      true,
		FailedSizes: make([]int, 0),
	}
}

func (m *MTUProbe) OnMTUResult(size int, ok bool, ms float64) {
	m.Total++
	m.LastSize = size
	m.LastMs = ms
	m.LastOK = ok

	if ok {
		if size > m.MaxSize {
			m.MaxSize = size
		}
	} else {
		m.Fail++
		// Track which sizes failed
		found := false
		for _, s := range m.FailedSizes {
			if s == size {
				found = true
				break
			}
		}
		if !found {
			m.FailedSizes = append(m.FailedSizes, size)
		}
		// If a smaller size fails, update max
		if size < m.MaxSize {
			m.MaxSize = size - 1
		}
	}
}

func (m *MTUProbe) Name() string    { return "MTU probe" }
func (m *MTUProbe) CSVFile() string { return "csv_mtu.csv" }
func (m *MTUProbe) Unit() string    { return "bytes" }
func (m *MTUProbe) Value() float64  { return float64(m.MaxSize) }
func (m *MTUProbe) IsOK() bool      { return m.LastOK && m.MaxSize >= 1400 }

func (m *MTUProbe) WarnThreshold() float64 { return 1400 } // below standard ethernet
func (m *MTUProbe) CritThreshold() float64 { return 1200 } // severely limited

func (m *MTUProbe) Score() Score {
	if !m.LastOK && m.MaxSize < 1200 {
		return Crit
	}
	if m.MaxSize < 1400 {
		return Warn
	}
	return Good
}

func (m *MTUProbe) DisplayValue() string {
	if m.Total == 0 {
		return "not tested"
	}
	if len(m.FailedSizes) > 0 {
		return fmt.Sprintf("%d bytes (fail@%v)", m.MaxSize, m.FailedSizes)
	}
	return fmt.Sprintf("%d bytes", m.MaxSize)
}

// DetectedMTU returns the effective MTU detected.
func (m *MTUProbe) DetectedMTU() int {
	return m.MaxSize
}

// HasFragmentation returns true if we detected MTU issues.
func (m *MTUProbe) HasFragmentation() bool {
	return len(m.FailedSizes) > 0 || m.MaxSize < 1500
}
