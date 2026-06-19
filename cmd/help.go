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
  nstat status [-l]
  nstat log
  nstat graph [--hours N]
  nstat -h | -v

%sOPTIONS (start)%s
  --interval N   seconds between ICMP pings (default: 5)
  --window N     number of pings used for RTT avg/jitter (default: 60)
                 e.g. window=60 with interval=5 → 5-minute rolling average

%sOPTIONS (status)%s
  -l, --lan      run LAN diagnostics: 50 samples of ICMP, TCP, DNS to
                 router, LAN hosts, and WAN targets to isolate issues

%sDIMENSIONS%s
  RTT (avg)      rolling average ICMP RTT to 8.8.8.8
                 Good: <80ms  Warn: 80–200ms  Crit: >200ms
  Jitter         std dev of RTT (same window)
                 Good: <10ms  Warn: 10–30ms  Crit: >30ms
  Packet loss    %% recent pings with no reply (sliding window)
                 Good: <1%%  Warn: 1–5%%  Crit: >5%%
  TCP connect    time for TCP handshake to 8.8.8.8:53
                 Good: <150ms  Crit: failed
  TCP loss       %% recent TCP attempts that timed out — refused/reset
                 excluded (sliding window)
                 Good: <1%%  Warn: 1–5%%  Crit: >5%%
  MTU            path MTU to 8.8.8.8, probed with the Don't-Fragment bit
                 Good: ≥1400  Warn: <1400  Crit: <1200
  DNS <ip>       time to resolve via your DNS server (auto re-detected)
                 Good: <100ms  Warn: 100–500ms  Crit: failed
  Gateway <ip>   ICMP ping to your default gateway, auto re-detected (LAN health)
                 Good: <10ms  Warn: 10–50ms  Crit: failed
  DHCP <ip>      current DHCP lease state — server + expiry read from the
                 system, not a ping (graph tracks time left). "n/a" if no source
                 Good: valid  Warn: renewing  Crit: expired
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
		"\033[1m", "\033[0m",
		cfg.GraphFile,
		cfg.Dir,
		"\033[1m", "\033[0m",
		cfg.Dir,
	)
}
