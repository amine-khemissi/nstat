package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/amine-khemissi/nstat/config"
	"github.com/amine-khemissi/nstat/dim"
	"github.com/amine-khemissi/nstat/state"
	"github.com/amine-khemissi/nstat/version"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[91m"
	colorGreen  = "\033[92m"
	colorYellow = "\033[93m"
	colorCyan   = "\033[96m"
	colorBold   = "\033[1m"
)

func Status() {
	cfg := config.Default()

	pid, err := readPID(cfg.PIDFile)
	if err != nil || !processAlive(pid) {
		fmt.Printf("%snstat is not running%s\n", colorRed, colorReset)
		fmt.Printf("  start it with:  %snstat start%s\n", colorBold, colorReset)
		os.Exit(1)
	}

	s, err := state.Read(cfg.StateFile)
	if err != nil {
		fmt.Printf("%sdaemon running but no state yet — wait a few seconds%s\n", colorYellow, colorReset)
		return
	}

	elapsed := time.Since(time.Unix(s.SessionStart, 0))
	rotateIn := cfg.LogRotateEvery - elapsed
	if rotateIn < 0 {
		rotateIn = 0
	}

	outages1h := countOutagesInWindow(s.RecentOutage, 3600)

	inOutageStr := ""
	if s.InOutage {
		inOutageStr = fmt.Sprintf("  %s[IN OUTAGE]%s", colorRed, colorReset)
	}

	sep := "─────────────────────────────────────────────────────────────"
	fmt.Println()
	fmt.Printf("  %snstat %s%s — pid %d  running %s  log rotates in %s%s\n",
		colorBold, version.String(), colorReset, pid, fmtDuration(elapsed), fmtDuration(rotateIn), inOutageStr)
	fmt.Printf("  last update: %s  (ping every %ds, checks every %ds, window: %d pings)\n",
		s.Timestamp, s.PingInterval, s.LANInterval, s.RTTWindow)
	fmt.Printf("  %s\n", sep)
	fmt.Printf("  %-22s  %-22s  %s\n", "dimension", "value", "score")
	fmt.Printf("  %s\n", sep)

	rows := []struct {
		name  string
		value string
		score dim.Score
	}{
		{"RTT (avg)", fmt.Sprintf("%.1f ms", s.RTTAvg), dim.ScoreOf(s.RTTAvg, true, 80, 200)},
		{"Jitter", fmt.Sprintf("%.1f ms", s.RTTJitter), dim.ScoreOf(s.RTTJitter, true, 10, 30)},
		{"Packet loss", fmt.Sprintf("%.1f%%  (%d/%d)", s.LossPct, s.LossTotal, s.PingsTotal), dim.ScoreOf(s.LossPct, true, 1, 5)},
	}

	// Add all TCP targets
	for i, t := range s.TCPTargets {
		name := fmt.Sprintf("TCP %s:%d", t.Host, t.Port)
		var value string
		if i == 0 {
			// Primary target shows more detail
			value = fmt.Sprintf("%.1f ms", t.LastMs)
		} else {
			value = fmt.Sprintf("%.1f ms  loss:%.1f%%", t.LastMs, t.LossPct)
		}
		rows = append(rows, struct {
			name  string
			value string
			score dim.Score
		}{name, value, dim.ScoreOf(t.LastMs, t.LastOK, 150, 150)})
	}

	// Fallback if no targets in state (old state format)
	if len(s.TCPTargets) == 0 {
		rows = append(rows, struct {
			name  string
			value string
			score dim.Score
		}{fmt.Sprintf("TCP %s:%d", cfg.TCPHost, cfg.TCPPort), fmt.Sprintf("%.1f ms", s.TCPLastMs), dim.ScoreOf(s.TCPLastMs, s.TCPLastOK, 150, 150)})
	}

	// TCP loss (aggregated)
	rows = append(rows, struct {
		name  string
		value string
		score dim.Score
	}{"TCP loss", fmt.Sprintf("%.1f%%  (%d/%d)", s.TCPLossPct, s.TCPFail, s.TCPTotal), dim.ScoreOf(s.TCPLossPct, true, 1, 5)})

	// TCP failure breakdown (always show)
	tcpBreakdown := fmt.Sprintf("to:%d ref:%d rst:%d oth:%d",
		s.TCPTimeoutCount, s.TCPRefusedCount, s.TCPResetCount, s.TCPOtherCount)
	tcpBreakdownScore := dim.Good
	if s.TCPTimeoutCount > 0 || s.TCPRefusedCount > 0 || s.TCPResetCount > 0 {
		tcpBreakdownScore = dim.Warn
	}
	rows = append(rows, struct {
		name  string
		value string
		score dim.Score
	}{"TCP breakdown", tcpBreakdown, tcpBreakdownScore})

	// MTU probe (always show)
	mtuValue := "not tested"
	mtuScore := dim.Good
	if s.MTUDetected > 0 {
		mtuValue = fmt.Sprintf("%d bytes", s.MTUDetected)
		if s.MTUHasIssues {
			mtuValue = fmt.Sprintf("%d bytes (fragmentation!)", s.MTUDetected)
			mtuScore = dim.Warn
		}
		if s.MTUDetected < 1400 {
			mtuScore = dim.Warn
		}
		if s.MTUDetected < 1200 {
			mtuScore = dim.Crit
		}
	}
	rows = append(rows, struct {
		name  string
		value string
		score dim.Score
	}{"MTU probe", mtuValue, mtuScore})

	// Continue with standard dimensions
	rows = append(rows, []struct {
		name  string
		value string
		score dim.Score
	}{
		{fmt.Sprintf("DNS %s", s.DNSServer), fmt.Sprintf("%.1f ms", s.DNSLastMs), dim.ScoreOf(s.DNSLastMs, s.DNSLastOK, 100, 500)},
		{fmt.Sprintf("DHCP %s", s.DHCPServer), fmt.Sprintf("%.1f ms", s.DHCPLastMs), dim.ScoreOf(s.DHCPLastMs, s.DHCPLastOK, 10, 50)},
		{"Outages/1h", fmt.Sprintf("%d  (%d total)", outages1h, s.OutageCount), dim.ScoreOf(float64(outages1h), true, 1, 3)},
	}...)

	// Kernel TCP stats (only show if we have data, since not all platforms support it)
	if s.KernelDeltaOutSegs > 0 || s.KernelRetransPct > 0 || s.KernelCurrEstab > 0 {
		retransValue := fmt.Sprintf("%.2f%%  (+%d/%d segs)", s.KernelRetransPct, s.KernelDeltaRetrans, s.KernelDeltaOutSegs)
		rows = append(rows, struct {
			name  string
			value string
			score dim.Score
		}{"TCP retrans", retransValue, dim.ScoreOf(s.KernelRetransPct, true, 2, 5)})

		if s.KernelDeltaInErrs > 0 || s.KernelDeltaResets > 0 {
			errValue := fmt.Sprintf("errs:%d rsts:%d estab:%d", s.KernelDeltaInErrs, s.KernelDeltaResets, s.KernelCurrEstab)
			errScore := dim.ScoreOf(float64(s.KernelDeltaInErrs+s.KernelDeltaResets), true, 1, 10)
			rows = append(rows, struct {
				name  string
				value string
				score dim.Score
			}{"TCP errors", errValue, errScore})
		}
	}

	for _, r := range rows {
		fmt.Printf("  %-22s  %-22s  %s\n", r.name, r.value, scoreColor(r.score))
	}

	// TCP tuning (sysctl settings) - shown separately with custom label
	if s.TCPSynRetries > 0 || s.TCPRetries2 > 0 {
		tuningValue := fmt.Sprintf("syn:%d ret:%d ka:%s",
			s.TCPSynRetries, s.TCPRetries2, fmtSeconds(s.TCPKeepaliveTime))
		var tuningDisplay string
		if s.TCPFastFail {
			tuningDisplay = colorGreen + "●  OK" + colorReset
		} else {
			tuningDisplay = colorYellow + "●  SLOW" + colorReset
		}
		fmt.Printf("  %-22s  %-22s  %s\n", "TCP tuning", tuningValue, tuningDisplay)
	}

	fmt.Printf("  %s\n", sep)
	overall := overallScore(s, outages1h)
	fmt.Printf("  %-22s  %-22s  %s\n", "Overall", "", scoreColor(overall))
	fmt.Printf("  %s\n", sep)
	fmt.Println()
}

func scoreColor(s dim.Score) string {
	switch s {
	case dim.Good:
		return colorGreen + "●  GOOD" + colorReset
	case dim.Warn:
		return colorYellow + "●  WARN" + colorReset
	default:
		return colorRed + "●  CRIT" + colorReset
	}
}

func overallScore(s *state.State, outages1h int) dim.Score {
	worst := dim.Good
	bump := func(sc dim.Score) {
		if sc > worst {
			worst = sc
		}
	}
	bump(dim.ScoreOf(s.RTTAvg, true, 80, 200))
	bump(dim.ScoreOf(s.RTTJitter, true, 10, 30))
	bump(dim.ScoreOf(s.LossPct, true, 1, 5))
	bump(dim.ScoreOf(s.TCPLastMs, s.TCPLastOK, 150, 150))
	bump(dim.ScoreOf(s.TCPLossPct, true, 1, 5))
	bump(dim.ScoreOf(s.DNSLastMs, s.DNSLastOK, 100, 500))
	bump(dim.ScoreOf(s.DHCPLastMs, s.DHCPLastOK, 10, 50))
	bump(dim.ScoreOf(float64(outages1h), true, 1, 3))
	return worst
}

func countOutagesInWindow(recent []int64, windowSecs int64) int {
	cutoff := time.Now().Unix() - windowSecs
	n := 0
	for _, ts := range recent {
		if ts >= cutoff {
			n++
		}
	}
	return n
}

func fmtDuration(d time.Duration) string {
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

func fmtSeconds(secs int) string {
	switch {
	case secs < 60:
		return fmt.Sprintf("%ds", secs)
	case secs < 3600:
		return fmt.Sprintf("%dm", secs/60)
	default:
		return fmt.Sprintf("%dh", secs/3600)
	}
}
