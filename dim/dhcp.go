package dim

import "fmt"

// Gateway measures reachability/latency of the default gateway via ICMP.
// (This was previously mislabeled "DHCP" — it never spoke DHCP; it pings the
// gateway. The real DHCP lease check lives in DHCPLease.)
type Gateway struct {
	server string
	lastMs float64
	lastOK bool
}

func NewGateway(server string) *Gateway {
	return &Gateway{server: server, lastOK: true}
}

func (d *Gateway) OnGatewayResult(ok bool, ms float64) {
	if ok {
		d.lastMs = ms
		d.lastOK = true
	} else {
		d.lastMs = 0
		d.lastOK = false
	}
}

func (d *Gateway) SetServer(s string) { d.server = s }
func (d *Gateway) Server() string     { return d.server }

func (d *Gateway) Name() string           { return fmt.Sprintf("Gateway %s", d.server) }
func (d *Gateway) CSVFile() string        { return "csv_gateway.csv" }
func (d *Gateway) Unit() string           { return "ms" }
func (d *Gateway) Value() float64         { return d.lastMs }
func (d *Gateway) IsOK() bool             { return d.lastOK }
func (d *Gateway) WarnThreshold() float64 { return 10 }
func (d *Gateway) CritThreshold() float64 { return 50 }
func (d *Gateway) Score() Score           { return ScoreOf(d.lastMs, d.lastOK, 10, 50) }
func (d *Gateway) DisplayValue() string {
	if !d.lastOK {
		return "unreachable"
	}
	return FmtMs(d.lastMs)
}
