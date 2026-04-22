package dim

import "fmt"

type DNS struct {
	server string
	lastMs float64
	lastOK bool
}

func NewDNS(server string) *DNS {
	return &DNS{server: server, lastOK: true}
}

func (d *DNS) OnDNSResult(ok bool, ms float64) {
	if ok {
		d.lastMs = ms
		d.lastOK = true
	} else {
		d.lastMs = 0
		d.lastOK = false
	}
}

func (d *DNS) SetServer(s string) { d.server = s }
func (d *DNS) Server() string     { return d.server }

func (d *DNS) Name() string           { return fmt.Sprintf("DNS %s", d.server) }
func (d *DNS) CSVFile() string        { return "csv_dns.csv" }
func (d *DNS) Unit() string           { return "ms" }
func (d *DNS) Value() float64         { return d.lastMs }
func (d *DNS) IsOK() bool             { return d.lastOK }
func (d *DNS) WarnThreshold() float64 { return 100 }
func (d *DNS) CritThreshold() float64 { return 500 }
func (d *DNS) Score() Score           { return ScoreOf(d.lastMs, d.lastOK, 100, 500) }
func (d *DNS) DisplayValue() string   { return FmtMs(d.lastMs) }
