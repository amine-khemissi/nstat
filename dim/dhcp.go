package dim

import "fmt"

type DHCP struct {
	server string
	lastMs float64
	lastOK bool
}

func NewDHCP(server string) *DHCP {
	return &DHCP{server: server, lastOK: true}
}

func (d *DHCP) OnDHCPResult(ok bool, ms float64) {
	if ok {
		d.lastMs = ms
		d.lastOK = true
	} else {
		d.lastMs = 0
		d.lastOK = false
	}
}

func (d *DHCP) SetServer(s string) { d.server = s }
func (d *DHCP) Server() string     { return d.server }

func (d *DHCP) Name() string           { return fmt.Sprintf("DHCP %s", d.server) }
func (d *DHCP) CSVFile() string        { return "csv_dhcp.csv" }
func (d *DHCP) Unit() string           { return "ms" }
func (d *DHCP) Value() float64         { return d.lastMs }
func (d *DHCP) IsOK() bool             { return d.lastOK }
func (d *DHCP) WarnThreshold() float64 { return 10 }
func (d *DHCP) CritThreshold() float64 { return 50 }
func (d *DHCP) Score() Score           { return ScoreOf(d.lastMs, d.lastOK, 10, 50) }
func (d *DHCP) DisplayValue() string   { return FmtMs(d.lastMs) }
