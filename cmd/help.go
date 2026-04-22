package cmd

import (
	"fmt"

	"github.com/amine-khemissi/nstat/config"
)

func Help() {
	cfg := config.Default()
	fmt.Printf(`
%snstat%s — network connection reliability monitor

%sUSAGE%s
  nstat start [--interval N] [--window N]
  nstat stop
  nstat status
  nstat log
  nstat graph [--hours N]
  nstat -h

%sOPTIONS (start)%s
  --interval N   seconds between ICMP pings (default: 5)
  --window N     number of pings used for RTT avg/jitter (default: 60)
                 e.g. window=60 with interval=5 → 5-minute rolling average

%sDIMENSIONS%s
  RTT (avg)      rolling average ICMP RTT to 8.8.8.8
                 Good: <80ms  Warn: 80–200ms  Crit: >200ms
  Jitter         std dev of RTT (same window)
                 Good: <10ms  Warn: 10–30ms  Crit: >30ms
  Packet loss    %% pings with no reply since start
                 Good: <1%%  Warn: 1–5%%  Crit: >5%%
  TCP connect    time for TCP handshake to 8.8.8.8:53
                 Good: <150ms  Crit: failed
  TCP loss       %% TCP attempts that failed
                 Good: <1%%  Warn: 1–5%%  Crit: >5%%
  DNS <ip>       time to resolve google.com via your DNS server
                 Good: <100ms  Warn: 100–500ms  Crit: failed
  DHCP <ip>      ICMP ping to your default gateway (LAN health)
                 Good: <10ms  Warn: 10–50ms  Crit: failed
  Outages/1h     distinct outage events (3+ consecutive losses) in the last hour
                 Good: 0  Warn: 1  Crit: ≥3
  Overall        worst score across all dimensions

%sGRAPH%s
  nstat graph              SVG chart of all dimensions (full history)
  nstat graph --hours N    limit to the last N hours
  Output: %s
  CSV data per dimension in: %s/csv_*.csv

%sLOG ROTATION%s
  Every 24h the daemon rotates nstat.log → .1 → .2 → .3 and resets counters.
  CSV files are NOT rotated — they accumulate for long-term trend analysis.
  Data directory: %s

`,
		"\033[1m", "\033[0m",
		"\033[1m", "\033[0m",
		"\033[1m", "\033[0m",
		"\033[1m", "\033[0m",
		"\033[1m", "\033[0m",
		cfg.GraphFile,
		cfg.Dir,
		"\033[1m", "\033[0m",
		cfg.Dir,
	)
}
