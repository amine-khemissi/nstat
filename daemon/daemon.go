package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"nstat/config"
	"nstat/dim"
	"nstat/state"
	"nstat/store"
)

// Run is the main daemon loop. It is called from cmd/daemon.go after the
// process has been detached into the background.
func Run(cfg *config.Config) {
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "nstat: mkdir %s: %v\n", cfg.Dir, err)
		os.Exit(1)
	}

	logF := openLog(cfg.LogFile)

	logf := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		ts := time.Now().Format("2006-01-02 15:04:05")
		line := fmt.Sprintf("[%s] %s\n", ts, msg)
		fmt.Fprint(logF, line)
	}

	// --- build dimensions ---------------------------------------------------
	ps := dim.NewPingStats(cfg.RTTWindow)
	rtt := dim.NewRTT(ps)
	jitter := dim.NewJitter(ps)
	pl, lossStats := dim.NewPacketLoss()
	tc, tcpStats := dim.NewTCPConnect(cfg.TCPHost, cfg.TCPPort)
	tcpLoss := dim.NewTCPLoss(tcpStats)
	dnsServer := detectDNS()
	dns := dim.NewDNS(dnsServer)
	gateway, err := detectGateway()
	if err != nil {
		gateway = ""
	}
	dhcp := dim.NewDHCP(gateway)
	outages := dim.NewOutages()

	pingObs := []dim.PingObserver{ps, lossStats}
	tcpObs := []dim.TCPObserver{tcpStats}
	dnsObs := []dim.DNSObserver{dns}
	dhcpObs := []dim.DHCPObserver{dhcp}

	dims := []dim.Dimension{rtt, jitter, pl, tc, tcpLoss, dns, dhcp, outages}

	// --- state snapshot used by `nstat status` ------------------------------
	snap := &state.State{
		SessionStart: time.Now().Unix(),
		PingInterval: int(cfg.PingInterval.Seconds()),
		LANInterval:  int(cfg.LANInterval.Seconds()),
		RTTWindow:    cfg.RTTWindow,
		DNSServer:    dnsServer,
		DHCPServer:   gateway,
		TCPLastOK:    true,
		DNSLastOK:    true,
		DHCPLastOK:   true,
	}

	// --- write PID file -----------------------------------------------------
	if err := os.WriteFile(cfg.PIDFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "nstat: write pid: %v\n", err)
		os.Exit(1)
	}

	logf("daemon started  ping:%s  interval:%ds  window:%d  tcp:%s:%d  dns:%s  dhcp:%s  pid:%d",
		cfg.PingTarget, int(cfg.PingInterval.Seconds()), cfg.RTTWindow,
		cfg.TCPHost, cfg.TCPPort, dnsServer, gateway, os.Getpid())

	// --- signal handler -----------------------------------------------------
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		printSummary(logf, snap, cfg)
		os.Remove(cfg.PIDFile)
		logF.Close()
		os.Exit(0)
	}()

	// --- initial LAN check --------------------------------------------------
	doLANChecks(cfg, dns, dhcp, tcpObs, dnsObs, dhcpObs, snap, logf, dims)
	lastLAN := time.Now()

	// --- main loop ----------------------------------------------------------
	logRotateAt := time.Now().Add(cfg.LogRotateEvery)
	consecutiveLoss := 0
	inOutage := false
	outageStart := time.Time{}

	for {
		// log rotation
		if time.Now().After(logRotateAt) {
			printSummary(logf, snap, cfg)
			logf("── log rotating ─────────────────────────────────────")
			logF.Close()
			rotateLogs(cfg.LogFile)
			logF = openLog(cfg.LogFile)
			logf = func(format string, args ...any) {
				msg := fmt.Sprintf(format, args...)
				ts := time.Now().Format("2006-01-02 15:04:05")
				line := fmt.Sprintf("[%s] %s\n", ts, msg)
				fmt.Fprint(logF, line)
			}
			// reset counters
			ps = dim.NewPingStats(cfg.RTTWindow)
			rtt = dim.NewRTT(ps)
			jitter = dim.NewJitter(ps)
			pl, lossStats = dim.NewPacketLoss()
			pingObs = []dim.PingObserver{ps, lossStats}
			dims[0], dims[1], dims[2] = rtt, jitter, pl
			snap.SessionStart = time.Now().Unix()
			snap.PingsTotal = 0
			snap.LossTotal = 0
			snap.TCPTotal = 0
			snap.TCPFail = 0
			snap.OutageCount = 0
			outages = dim.NewOutages()
			dims[7] = outages
			consecutiveLoss = 0
			inOutage = false
			logRotateAt = time.Now().Add(cfg.LogRotateEvery)
			logf("daemon resumed after log rotation  pid:%d", os.Getpid())
		}

		loopStart := time.Now()

		// ── ICMP ping ──────────────────────────────────────────────────────
		rttMs, err := pingOnce(cfg.PingTarget, 2*time.Second)

		snap.PingsTotal++
		if err != nil {
			snap.LossTotal++
			consecutiveLoss++
			snap.RTTCurrent = 0
			for _, o := range pingObs {
				o.OnPingFailure()
			}
			if consecutiveLoss == cfg.LossThreshold {
				inOutage = true
				outageStart = time.Now()
				outages.RecordOutage(outageStart.Unix())
				snap.OutageCount = outages.Count
				snap.InOutage = true

				rtt2, err2 := pingOnce(cfg.PingTarget2, 2*time.Second)
				if err2 != nil {
					logf("OUTAGE #%d — %s and %s both unreachable",
						outages.Count, cfg.PingTarget, cfg.PingTarget2)
				} else {
					logf("OUTAGE #%d — %s down, %s OK (%.0fms)",
						outages.Count, cfg.PingTarget, cfg.PingTarget2, rtt2)
				}
			} else if consecutiveLoss < cfg.LossThreshold {
				logf("loss %d/%d  [%s]", consecutiveLoss, cfg.LossThreshold, cfg.PingTarget)
			}
		} else {
			if inOutage {
				dur := time.Since(outageStart)
				logf("RECOVERY — outage lasted %s, RTT now %.0fms", fmtDur(dur), rttMs)
				inOutage = false
				outageStart = time.Time{}
				outages.RecordRecovery()
				snap.InOutage = false
			}
			consecutiveLoss = 0
			snap.RTTCurrent = rttMs
			for _, o := range pingObs {
				o.OnPingSuccess(rttMs)
			}
		}

		snap.RTTAvg = ps.Avg
		snap.RTTJitter = ps.Jitter
		snap.LossPct = pl.Value()

		// CSV: ping-rate dimensions (every ping interval)
		now := time.Now()
		if rttMs > 0 {
			appendCSV(cfg, dims[0], now) // rtt
			appendCSV(cfg, dims[1], now) // jitter
		}
		appendCSV(cfg, dims[2], now) // packet loss

		// ── LAN checks every LANInterval ───────────────────────────────────
		if time.Since(lastLAN) >= cfg.LANInterval {
			doLANChecks(cfg, dns, dhcp, tcpObs, dnsObs, dhcpObs, snap, logf, dims)
			lastLAN = time.Now()
		}

		snap.RecentOutage = outages.RecentTimes()
		if err := state.Write(cfg.StateFile, snap); err != nil {
			logf("state write error: %v", err)
		}

		elapsed := time.Since(loopStart)
		if sleep := cfg.PingInterval - elapsed; sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

func doLANChecks(
	cfg *config.Config,
	dns *dim.DNS, dhcp *dim.DHCP,
	tcpObs []dim.TCPObserver, dnsObs []dim.DNSObserver, dhcpObs []dim.DHCPObserver,
	snap *state.State,
	logf func(string, ...any),
	dims []dim.Dimension,
) {
	now := time.Now()

	// TCP
	tcpMs, err := tcpCheck(cfg.TCPHost, cfg.TCPPort, 2*time.Second)
	ok := err == nil
	for _, o := range tcpObs {
		o.OnTCPResult(ok, tcpMs)
	}
	snap.TCPLastMs = dims[3].Value()
	snap.TCPLastOK = ok
	snap.TCPTotal++
	if !ok {
		snap.TCPFail++
		logf("TCP FAIL  %s:%d  (%d/%d)", cfg.TCPHost, cfg.TCPPort, snap.TCPFail, snap.TCPTotal)
	} else {
		snap.TCPLastMs = tcpMs
		logf("TCP OK    %s:%d  connect=%.0fms", cfg.TCPHost, cfg.TCPPort, tcpMs)
	}
	if snap.TCPTotal > 0 {
		snap.TCPLossPct = float64(snap.TCPFail) / float64(snap.TCPTotal) * 100
	}
	appendCSV(cfg, dims[3], now) // tcp connect
	appendCSV(cfg, dims[4], now) // tcp loss

	// DNS
	dnsMs, err := dnsCheck(dns.Server(), 2*time.Second)
	ok = err == nil
	for _, o := range dnsObs {
		o.OnDNSResult(ok, dnsMs)
	}
	snap.DNSLastMs = dims[5].Value()
	snap.DNSLastOK = ok
	if !ok {
		logf("DNS FAIL  %s", dns.Server())
	} else {
		logf("DNS OK    %s  resolve=%.0fms", dns.Server(), dnsMs)
	}
	appendCSV(cfg, dims[5], now) // dns

	// DHCP
	if dhcp.Server() != "" {
		dhcpMs, err := pingOnce(dhcp.Server(), 2*time.Second)
		ok = err == nil
		for _, o := range dhcpObs {
			o.OnDHCPResult(ok, dhcpMs)
		}
		snap.DHCPLastMs = dims[6].Value()
		snap.DHCPLastOK = ok
		if !ok {
			logf("DHCP FAIL %s", dhcp.Server())
		} else {
			logf("DHCP OK   %s  ping=%.0fms", dhcp.Server(), dhcpMs)
		}
	}
	appendCSV(cfg, dims[6], now) // dhcp
}

func appendCSV(cfg *config.Config, d dim.Dimension, t time.Time) {
	if d.CSVFile() == "" {
		return
	}
	path := filepath.Join(cfg.Dir, d.CSVFile())
	_ = store.Append(path, d.CSVFile()[:len(d.CSVFile())-4], t, d.Value())
}

func printSummary(logf func(string, ...any), snap *state.State, cfg *config.Config) {
	elapsed := time.Since(time.Unix(snap.SessionStart, 0))
	logf("── session summary ──────────────────────────────────")
	logf("  duration    : %s", fmtDur(elapsed))
	logf("  total pings : %d", snap.PingsTotal)
	logf("  packet loss : %d/%d (%.1f%%)", snap.LossTotal, snap.PingsTotal, snap.LossPct)
	logf("  avg RTT     : %.1f ms", snap.RTTAvg)
	logf("  jitter      : %.1f ms", snap.RTTJitter)
	logf("  tcp checks  : %d/%d failed (%.1f%%)", snap.TCPFail, snap.TCPTotal, snap.TCPLossPct)
	logf("  outages     : %d", snap.OutageCount)
}

func fmtDur(d time.Duration) string {
	s := d.Seconds()
	switch {
	case s < 60:
		return fmt.Sprintf("%.0fs", s)
	case s < 3600:
		return fmt.Sprintf("%.1fmin", s/60)
	default:
		return fmt.Sprintf("%.2fh", s/3600)
	}
}

func openLog(path string) *os.File {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nstat: open log %s: %v\n", path, err)
		os.Exit(1)
	}
	return f
}

func rotateLogs(logFile string) {
	for i := 2; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", logFile, i)
		dst := fmt.Sprintf("%s.%d", logFile, i+1)
		os.Rename(src, dst)
	}
	os.Rename(logFile, logFile+".1")
}
