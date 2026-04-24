package cmd

import (
	"fmt"
	"math"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const lanDiagSamples = 50

type diagResult struct {
	Name    string
	Proto   string
	OK      int
	Fail    int
	LossPct float64
	Avg     float64
	Min     float64
	Max     float64
	StdDev  float64
}

func (r *diagResult) Score() string {
	if r.LossPct == 0 && r.OK > 0 {
		return colorGreen + "●" + colorReset
	}
	if r.LossPct < 5 {
		return colorYellow + "●" + colorReset
	}
	return colorRed + "●" + colorReset
}

// RunLANDiag runs comprehensive LAN diagnostics
func RunLANDiag() {
	fmt.Println()
	fmt.Printf("  %sLAN Diagnostic%s — %d samples per test\n", colorBold, colorReset, lanDiagSamples)
	fmt.Println("  " + strings.Repeat("─", 95))
	fmt.Printf("  %-6s %-22s %5s %5s %6s %9s %9s %9s %9s  %s\n",
		"proto", "target", "ok", "fail", "loss", "avg", "min", "max", "σ", "")
	fmt.Println("  " + strings.Repeat("─", 95))

	gateway := detectGateway()
	results := []diagResult{}

	// LAN tests
	fmt.Printf("  %s── LAN ──%s\n", colorCyan, colorReset)
	if gateway != "" {
		results = append(results, testICMP(gateway, "Router"))
		results = append(results, testTCP(gateway, 80, "Router HTTP"))
		results = append(results, testDNS(gateway, "Router DNS"))
	}

	// Find other LAN hosts
	lanHosts := discoverLANHosts(gateway)
	for _, host := range lanHosts {
		results = append(results, testICMP(host, "LAN "+host))
	}

	// WAN tests
	fmt.Printf("  %s── WAN ──%s\n", colorCyan, colorReset)
	results = append(results, testICMP("8.8.8.8", "Google"))
	results = append(results, testICMP("1.1.1.1", "Cloudflare"))
	results = append(results, testTCP("8.8.8.8", 53, "Google DNS"))
	results = append(results, testTCP("8.8.8.8", 443, "Google HTTPS"))
	results = append(results, testTCP("1.1.1.1", 53, "Cloudflare DNS"))
	results = append(results, testTCP("1.1.1.1", 443, "Cloudflare HTTPS"))
	results = append(results, testDNS("8.8.8.8", "Google DNS"))
	results = append(results, testDNS("1.1.1.1", "Cloudflare DNS"))

	fmt.Println("  " + strings.Repeat("─", 95))
	fmt.Println()

	// Analysis
	lanLoss := 0.0
	wanICMPLoss := 0.0
	wanTCPLoss := 0.0
	lanCount := 0
	wanICMPCount := 0
	wanTCPCount := 0

	for _, r := range results {
		if strings.HasPrefix(r.Name, "Router") || strings.HasPrefix(r.Name, "LAN") {
			lanLoss += r.LossPct
			lanCount++
		} else if r.Proto == "ICMP" {
			wanICMPLoss += r.LossPct
			wanICMPCount++
		} else if r.Proto == "TCP" {
			wanTCPLoss += r.LossPct
			wanTCPCount++
		}
	}

	if lanCount > 0 {
		lanLoss /= float64(lanCount)
	}
	if wanICMPCount > 0 {
		wanICMPLoss /= float64(wanICMPCount)
	}
	if wanTCPCount > 0 {
		wanTCPLoss /= float64(wanTCPCount)
	}

	fmt.Printf("  %sAnalysis%s\n", colorBold, colorReset)
	fmt.Println()

	if lanLoss < 1 && wanTCPLoss > 1 {
		fmt.Printf("  %s→ LAN is healthy (<1%% loss), problem is router/ISP%s\n", colorGreen, colorReset)
	}
	if lanLoss >= 1 {
		fmt.Printf("  %s→ LAN has issues (%.1f%% avg loss) — check switch/cabling%s\n", colorYellow, lanLoss, colorReset)
	}
	if wanTCPLoss > wanICMPLoss*2 && wanTCPLoss > 1 {
		fmt.Printf("  %s→ WAN TCP loss (%.1f%%) >> ICMP loss (%.1f%%) — NAT/firewall throttling%s\n",
			colorYellow, wanTCPLoss, wanICMPLoss, colorReset)
	}
	if wanICMPLoss > 5 {
		fmt.Printf("  %s→ High ICMP loss (%.1f%%) — ISP or upstream issue%s\n", colorRed, wanICMPLoss, colorReset)
	}
	if lanLoss == 0 && wanICMPLoss == 0 && wanTCPLoss == 0 {
		fmt.Printf("  %s→ All tests passed — network is healthy%s\n", colorGreen, colorReset)
	}
	fmt.Println()
}

func testICMP(host, name string) diagResult {
	r := diagResult{Name: name, Proto: "ICMP"}
	times := []float64{}

	fmt.Printf("\r  %-6s %-22s testing...          ", "ICMP", name)

	for i := 0; i < lanDiagSamples; i++ {
		out, err := exec.Command("ping", "-c", "1", "-W", "1", host).CombinedOutput()
		if err != nil {
			r.Fail++
			continue
		}
		// Parse "time=X.XX ms"
		s := string(out)
		if idx := strings.Index(s, "time="); idx >= 0 {
			var ms float64
			fmt.Sscanf(s[idx:], "time=%f", &ms)
			times = append(times, ms)
			r.OK++
		} else {
			r.Fail++
		}
	}

	r.LossPct = float64(r.Fail) / float64(lanDiagSamples) * 100
	r.Avg, r.Min, r.Max, r.StdDev = calcStats(times)
	printResult(r)
	return r
}

func testTCP(host string, port int, name string) diagResult {
	r := diagResult{Name: name, Proto: "TCP"}
	times := []float64{}

	fmt.Printf("\r  %-6s %-22s testing...          ", "TCP", name)

	for i := 0; i < lanDiagSamples; i++ {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Second)
		if err != nil {
			r.Fail++
			continue
		}
		conn.Close()
		ms := float64(time.Since(start).Microseconds()) / 1000
		times = append(times, ms)
		r.OK++
	}

	r.LossPct = float64(r.Fail) / float64(lanDiagSamples) * 100
	r.Avg, r.Min, r.Max, r.StdDev = calcStats(times)
	printResult(r)
	return r
}

func testDNS(server, name string) diagResult {
	r := diagResult{Name: name, Proto: "DNS"}
	times := []float64{}

	fmt.Printf("\r  %-6s %-22s testing...          ", "DNS", name)

	for i := 0; i < lanDiagSamples; i++ {
		start := time.Now()
		// Use dig for DNS timing (more reliable across platforms)
		cmd := exec.Command("dig", "+short", "+time=1", "+tries=1", "@"+server, "google.com")
		err := cmd.Run()
		if err != nil {
			r.Fail++
			continue
		}
		ms := float64(time.Since(start).Microseconds()) / 1000
		times = append(times, ms)
		r.OK++
	}

	r.LossPct = float64(r.Fail) / float64(lanDiagSamples) * 100
	r.Avg, r.Min, r.Max, r.StdDev = calcStats(times)
	printResult(r)
	return r
}

func calcStats(times []float64) (avg, min, max, stddev float64) {
	if len(times) == 0 {
		return 0, 0, 0, 0
	}

	sort.Float64s(times)
	min = times[0]
	max = times[len(times)-1]

	sum := 0.0
	for _, t := range times {
		sum += t
	}
	avg = sum / float64(len(times))

	sumSq := 0.0
	for _, t := range times {
		sumSq += (t - avg) * (t - avg)
	}
	stddev = math.Sqrt(sumSq / float64(len(times)))

	return
}

func printResult(r diagResult) {
	avgStr, minStr, maxStr, stdStr := "N/A", "N/A", "N/A", "N/A"
	if r.OK > 0 {
		avgStr = fmt.Sprintf("%.2fms", r.Avg)
		minStr = fmt.Sprintf("%.2fms", r.Min)
		maxStr = fmt.Sprintf("%.2fms", r.Max)
		stdStr = fmt.Sprintf("%.2fms", r.StdDev)
	}

	// Clear the line first, then print result
	fmt.Printf("\r%-100s\r", "")
	fmt.Printf("  %-6s %-22s %5d %5d %5.1f%% %9s %9s %9s %9s  %s\n",
		r.Proto, r.Name, r.OK, r.Fail, r.LossPct, avgStr, minStr, maxStr, stdStr, r.Score())
}

func detectGateway() string {
	out, err := exec.Command("ip", "route").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "default") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2]
			}
		}
	}
	return ""
}

func discoverLANHosts(gateway string) []string {
	out, err := exec.Command("ip", "neigh", "show").Output()
	if err != nil {
		return nil
	}

	hosts := []string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		ip := fields[0]
		state := fields[len(fields)-1]
		if ip == gateway {
			continue
		}
		if !strings.HasPrefix(ip, "192.168") && !strings.HasPrefix(ip, "10.") && !strings.HasPrefix(ip, "172.") {
			continue
		}
		if state == "REACHABLE" || state == "STALE" || state == "DELAY" {
			hosts = append(hosts, ip)
		}
		if len(hosts) >= 3 {
			break
		}
	}
	return hosts
}
