package cmd

import (
	"fmt"
	"os"
	"time"

	"nstat/config"
	"nstat/dim"
	"nstat/state"
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
	fmt.Printf("  %snstat%s — pid %d  running %s  log rotates in %s%s\n",
		colorBold, colorReset, pid, fmtDuration(elapsed), fmtDuration(rotateIn), inOutageStr)
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
		{fmt.Sprintf("TCP %s:%d", cfg.TCPHost, cfg.TCPPort), fmt.Sprintf("%.1f ms", s.TCPLastMs), dim.ScoreOf(s.TCPLastMs, s.TCPLastOK, 150, 150)},
		{"TCP loss", fmt.Sprintf("%.1f%%  (%d/%d)", s.TCPLossPct, s.TCPFail, s.TCPTotal), dim.ScoreOf(s.TCPLossPct, true, 1, 5)},
		{fmt.Sprintf("DNS %s", s.DNSServer), fmt.Sprintf("%.1f ms", s.DNSLastMs), dim.ScoreOf(s.DNSLastMs, s.DNSLastOK, 100, 500)},
		{fmt.Sprintf("DHCP %s", s.DHCPServer), fmt.Sprintf("%.1f ms", s.DHCPLastMs), dim.ScoreOf(s.DHCPLastMs, s.DHCPLastOK, 10, 50)},
		{"Outages/1h", fmt.Sprintf("%d  (%d total)", outages1h, s.OutageCount), dim.ScoreOf(float64(outages1h), true, 1, 3)},
	}

	for _, r := range rows {
		fmt.Printf("  %-22s  %-22s  %s\n", r.name, r.value, scoreColor(r.score))
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
