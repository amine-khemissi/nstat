package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/amine-khemissi/nstat/config"
	"github.com/amine-khemissi/nstat/dim"
	"github.com/amine-khemissi/nstat/state"
	"github.com/amine-khemissi/nstat/store"
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

	// Multi-target TCP tracking
	tcpTargets := make([]struct{ Host string; Port int }, len(cfg.TCPTargets))
	for i, t := range cfg.TCPTargets {
		tcpTargets[i] = struct{ Host string; Port int }{t.Host, t.Port}
	}
	tcpMulti := dim.NewTCPMulti(tcpTargets)

	// MTU probe
	mtuProbeD := dim.NewMTUProbe()

	// Kernel TCP stats
	kernelStats := dim.NewKernelTCPStats()
	tcpRetrans := dim.NewTCPRetransmits(kernelStats)
	tcpErrors := dim.NewTCPErrors(kernelStats)

	pingObs := []dim.PingObserver{ps, lossStats}
	tcpObs := []dim.TCPObserver{tcpStats}
	dnsObs := []dim.DNSObserver{dns}
	dhcpObs := []dim.DHCPObserver{dhcp}

	dims := []dim.Dimension{rtt, jitter, pl, tc, tcpLoss, dns, dhcp, outages, tcpRetrans, tcpErrors}

	// --- state snapshot used by `nstat status` ------------------------------
	// Initialize per-target state
	tcpTargetStates := make([]state.TCPTargetState, len(cfg.TCPTargets))
	for i, t := range cfg.TCPTargets {
		tcpTargetStates[i] = state.TCPTargetState{
			Host:   t.Host,
			Port:   t.Port,
			LastOK: true,
		}
	}

	// Read TCP tuning settings
	tcpTuning, _ := ReadTCPTuning()

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
		TCPTargets:   tcpTargetStates,
	}

	// Store TCP tuning in state
	if tcpTuning != nil {
		snap.TCPSynRetries = tcpTuning.SynRetries
		snap.TCPRetries2 = tcpTuning.Retries2
		snap.TCPKeepaliveTime = tcpTuning.KeepaliveTime
		snap.TCPKeepaliveIntvl = tcpTuning.KeepaliveIntvl
		snap.TCPKeepaliveProbes = tcpTuning.KeepaliveProbes
		snap.TCPFinTimeout = tcpTuning.FinTimeout
		snap.TCPFastFail = tcpTuning.IsFastFail()
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
	doLANChecks(cfg, dns, dhcp, tcpObs, dnsObs, dhcpObs, snap, logf, dims, tcpMulti, mtuProbeD, kernelStats)
	lastLAN := time.Now()

	// --- main loop ----------------------------------------------------------
	logRotateAt := time.Now().Add(cfg.LogRotateEvery)
	consecutiveLoss := 0
	inOutage := false
	outageStart := time.Time{}

	for {
		// log + CSV rotation
		if time.Now().After(logRotateAt) {
			printSummary(logf, snap, cfg)
			logf("── log + CSV rotating ───────────────────────────────")
			logF.Close()
			rotateLogs(cfg.LogFile)
			if err := store.RotateCSVs(cfg.Dir); err != nil {
				// will log after reopening
			}
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
			doLANChecks(cfg, dns, dhcp, tcpObs, dnsObs, dhcpObs, snap, logf, dims, tcpMulti, mtuProbeD, kernelStats)
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
	tcpMulti *dim.TCPMulti,
	mtuProbeD *dim.MTUProbe,
	kernelStats *dim.KernelTCPStats,
) {
	now := time.Now()

	// --- TCP checks for all targets ---
	snap.TCPTimeoutCount = 0
	snap.TCPRefusedCount = 0
	snap.TCPResetCount = 0
	snap.TCPOtherCount = 0

	for i, target := range cfg.TCPTargets {
		tcpMs, reason, err := tcpCheckWithReason(target.Host, target.Port, 2*time.Second)
		ok := err == nil

		// Update multi-target tracker
		tcpMulti.RecordResult(target.Host, target.Port, ok, tcpMs, reason)

		// Update per-target state
		if i < len(snap.TCPTargets) {
			t := tcpMulti.GetTarget(target.Host, target.Port)
			if t != nil {
				snap.TCPTargets[i].LastMs = t.Stats.LastMs
				snap.TCPTargets[i].LastOK = t.Stats.LastOK
				snap.TCPTargets[i].LastReason = t.Stats.LastReason.String()
				snap.TCPTargets[i].Total = t.Stats.Total
				snap.TCPTargets[i].Fail = t.Stats.Fail
				snap.TCPTargets[i].LossPct = t.Stats.LossPct
				snap.TCPTargets[i].TimeoutCount = t.Stats.TimeoutCount
				snap.TCPTargets[i].RefusedCount = t.Stats.RefusedCount
				snap.TCPTargets[i].ResetCount = t.Stats.ResetCount
				snap.TCPTargets[i].OtherCount = t.Stats.OtherCount

				// Aggregate failure counts
				snap.TCPTimeoutCount += t.Stats.TimeoutCount
				snap.TCPRefusedCount += t.Stats.RefusedCount
				snap.TCPResetCount += t.Stats.ResetCount
				snap.TCPOtherCount += t.Stats.OtherCount
			}
		}

		if !ok {
			logf("TCP FAIL  %s:%d  reason=%s  (%v)", target.Host, target.Port, reason, err)
		} else {
			logf("TCP OK    %s:%d  connect=%.0fms", target.Host, target.Port, tcpMs)
		}

		// Primary target updates main state
		if i == 0 {
			for _, o := range tcpObs {
				o.OnTCPResult(ok, tcpMs)
			}
			snap.TCPLastMs = tcpMs
			snap.TCPLastOK = ok
			snap.TCPLastReason = reason.String()
			snap.TCPTotal++
			if !ok {
				snap.TCPFail++
			}
			if snap.TCPTotal > 0 {
				snap.TCPLossPct = float64(snap.TCPFail) / float64(snap.TCPTotal) * 100
			}
		}
	}
	appendCSV(cfg, dims[3], now) // tcp connect
	appendCSV(cfg, dims[4], now) // tcp loss

	// --- MTU Probe ---
	if cfg.MTUEnabled {
		mtuSize, mtuMs, err := mtuProbe(cfg.PingTarget, 2*time.Second)
		if err != nil {
			logf("MTU PROBE failed: %v", err)
			mtuProbeD.OnMTUResult(0, false, 0)
		} else {
			mtuProbeD.OnMTUResult(mtuSize, true, mtuMs)
			if mtuProbeD.HasFragmentation() {
				logf("MTU WARN  detected=%d  failed_sizes=%v", mtuSize, mtuProbeD.FailedSizes)
			} else {
				logf("MTU OK    detected=%d  latency=%.0fms", mtuSize, mtuMs)
			}
		}
		snap.MTUDetected = mtuProbeD.DetectedMTU()
		snap.MTULastMs = mtuProbeD.LastMs
		snap.MTUHasIssues = mtuProbeD.HasFragmentation()
		snap.MTUFailedSizes = mtuProbeD.FailedSizes
	}

	// --- Kernel TCP Stats ---
	if cfg.KernelStats {
		retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab, err := ReadKernelTCPStats()
		if err != nil {
			logf("KERNEL STATS failed: %v", err)
		} else {
			kernelStats.Update(retrans, outSegs, inSegs, inErrs, outRsts, attemptFails, estabResets, currEstab)
			snap.KernelRetransPct = kernelStats.RetransPct
			snap.KernelDeltaRetrans = kernelStats.DeltaRetrans
			snap.KernelDeltaOutSegs = kernelStats.DeltaOutSegs
			snap.KernelDeltaInErrs = kernelStats.DeltaInErrs
			snap.KernelDeltaResets = kernelStats.DeltaEstabResets
			snap.KernelCurrEstab = kernelStats.CurrEstab

			if kernelStats.RetransPct > 2 {
				logf("KERNEL WARN retrans=%.2f%% (+%d segs)", kernelStats.RetransPct, kernelStats.DeltaRetrans)
			} else {
				logf("KERNEL OK   retrans=%.2f%% estab=%d", kernelStats.RetransPct, kernelStats.CurrEstab)
			}
		}
		appendCSV(cfg, dims[8], now) // tcp retrans
		appendCSV(cfg, dims[9], now) // tcp errors
	}

	// DNS
	dnsMs, err := dnsCheck(dns.Server(), 2*time.Second)
	ok := err == nil
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
