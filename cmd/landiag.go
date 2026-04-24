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
		name := formatLANHost(host)
		results = append(results, testICMP(host.IP, name))
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

type lanHost struct {
	IP   string
	MAC  string
	Name string
}

func discoverLANHosts(gateway string) []lanHost {
	out, err := exec.Command("ip", "neigh", "show").Output()
	if err != nil {
		return nil
	}

	hosts := []lanHost{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
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
		if state != "REACHABLE" && state != "STALE" && state != "DELAY" {
			continue
		}

		// Find MAC address (usually after "lladdr")
		mac := ""
		for i, f := range fields {
			if f == "lladdr" && i+1 < len(fields) {
				mac = fields[i+1]
				break
			}
		}

		name := resolveHostname(ip, mac)
		hosts = append(hosts, lanHost{IP: ip, MAC: mac, Name: name})

		if len(hosts) >= 5 {
			break
		}
	}
	return hosts
}

// resolveHostname tries multiple methods to get a hostname for an IP
func resolveHostname(ip, mac string) string {
	// Try reverse DNS first (fastest)
	if name := tryReverseDNS(ip); name != "" {
		return name
	}

	// Try mDNS/Avahi (for .local names)
	if name := tryMDNS(ip); name != "" {
		return name
	}

	// Try NetBIOS (for Windows devices)
	if name := tryNetBIOS(ip); name != "" {
		return name
	}

	// Fall back to MAC vendor
	if mac != "" {
		if vendor := macVendor(mac); vendor != "" {
			return vendor
		}
	}

	return ""
}

func tryReverseDNS(ip string) string {
	out, err := exec.Command("host", "-W", "1", ip).Output()
	if err != nil {
		return ""
	}
	// Parse "X.X.X.X.in-addr.arpa domain name pointer hostname."
	s := string(out)
	if idx := strings.Index(s, "domain name pointer "); idx >= 0 {
		name := strings.TrimSpace(s[idx+20:])
		name = strings.TrimSuffix(name, ".")
		// Skip generic PTR records
		if strings.Contains(name, "in-addr.arpa") {
			return ""
		}
		return shortenName(name)
	}
	return ""
}

func tryMDNS(ip string) string {
	out, err := exec.Command("avahi-resolve", "-a", ip).Output()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) >= 2 {
		return shortenName(fields[1])
	}
	return ""
}

func tryNetBIOS(ip string) string {
	out, err := exec.Command("nmblookup", "-A", ip).Output()
	if err != nil {
		return ""
	}
	// Parse output for <00> unique name
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "<00>") && strings.Contains(line, "UNIQUE") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return ""
}

func shortenName(name string) string {
	// Remove common suffixes
	name = strings.TrimSuffix(name, ".local")
	name = strings.TrimSuffix(name, ".lan")
	name = strings.TrimSuffix(name, ".home")
	name = strings.TrimSuffix(name, ".internal")
	// Truncate if too long
	if len(name) > 12 {
		name = name[:9] + "..."
	}
	return name
}

func formatLANHost(host lanHost) string {
	// Get last octet of IP for short display
	parts := strings.Split(host.IP, ".")
	shortIP := host.IP
	if len(parts) == 4 {
		shortIP = "." + parts[3]
	}

	if host.Name != "" {
		// Show name with short IP
		name := host.Name
		if len(name) > 14 {
			name = name[:11] + "..."
		}
		return fmt.Sprintf("%s %s", name, shortIP)
	}
	return host.IP
}

// macVendor returns a short vendor name based on MAC OUI prefix
func macVendor(mac string) string {
	mac = strings.ToUpper(strings.ReplaceAll(mac, ":", ""))
	if len(mac) < 6 {
		return ""
	}
	oui := mac[:6]

	// Common OUI prefixes (first 3 bytes of MAC)
	vendors := map[string]string{
		"00037A": "Taiyo",
		"000C29": "VMware",
		"001132": "Synology",
		"0017F2": "Apple",
		"0019E3": "Apple",
		"001B63": "Apple",
		"001CB3": "Apple",
		"001D4F": "Apple",
		"001E52": "Apple",
		"001EC2": "Apple",
		"001F5B": "Apple",
		"001FF3": "Apple",
		"0021E9": "Apple",
		"002241": "Apple",
		"002312": "Apple",
		"002332": "Apple",
		"002436": "Apple",
		"00254B": "Apple",
		"0025BC": "Apple",
		"002608": "Apple",
		"00264A": "Apple",
		"0026B0": "Apple",
		"0026BB": "Apple",
		"003065": "Apple",
		"003EE1": "Apple",
		"0050E4": "Apple",
		"005882": "Apple",
		"006171": "Apple",
		"00A040": "Apple",
		"00B362": "Apple",
		"00C610": "Apple",
		"00CDFE": "Apple",
		"00F4B9": "Apple",
		"00F76F": "Apple",
		"041552": "Apple",
		"042665": "Apple",
		"045453": "Apple",
		"0C4DE9": "Apple",
		"0C771A": "Apple",
		"0CBC9F": "Apple",
		"1040F3": "Apple",
		"10417F": "Apple",
		"1094BB": "Apple",
		"109ADD": "Apple",
		"18AF8F": "Apple",
		"18E7F4": "Apple",
		"1C1AC0": "Apple",
		"244B03": "Apple",
		"28A02B": "Apple",
		"28CFDA": "Apple",
		"2CB43A": "Apple",
		"34363B": "Apple",
		"3C0754": "Apple",
		"3C15C2": "Apple",
		"3CE072": "Apple",
		"40331A": "Apple",
		"403004": "Apple",
		"442A60": "Apple",
		"44D884": "Apple",
		"48437C": "Apple",
		"48D705": "Apple",
		"4C32F5": "Apple",
		"4C8D79": "Apple",
		"503237": "Apple",
		"5433CB": "Apple",
		"549F13": "Apple",
		"54E43A": "Apple",
		"5855CA": "Apple",
		"587F57": "Apple",
		"5C5948": "Apple",
		"5C969D": "Apple",
		"5CCFCF": "Apple",
		"60C5AD": "Apple",
		"60D9C7": "Apple",
		"60F81D": "Apple",
		"640980": "Apple",
		"68D93C": "Apple",
		"6C4008": "Apple",
		"6C709F": "Apple",
		"6C94F8": "Apple",
		"70EF00": "Apple",
		"749EAF": "Apple",
		"78A3E4": "Apple",
		"7C0191": "Apple",
		"7CC3A1": "Apple",
		"7CD1C3": "Apple",
		"848506": "Apple",
		"84788B": "Apple",
		"848E0C": "Apple",
		"84FCFE": "Apple",
		"88C663": "Apple",
		"8C5877": "Apple",
		"8C8590": "Apple",
		"8CFABA": "Apple",
		"90840D": "Apple",
		"90B21F": "Apple",
		"90B931": "Apple",
		"98B8E3": "Apple",
		"98D6BB": "Apple",
		"98E0D9": "Apple",
		"9C04EB": "Apple",
		"9C207B": "Apple",
		"9C35EB": "Apple",
		"A01828": "Apple",
		"A03BE3": "Apple",
		"A0999B": "Apple",
		"A42305": "Apple",
		"A45E60": "Apple",
		"A4B197": "Apple",
		"A4D18C": "Apple",
		"A82066": "Apple",
		"A8968A": "Apple",
		"AC293A": "Apple",
		"ACFDEC": "Apple",
		"B065BD": "Apple",
		"B0702D": "Apple",
		"B48B19": "Apple",
		"B8098A": "Apple",
		"B817C2": "Apple",
		"B8C75D": "Apple",
		"B8E856": "Apple",
		"B8F6B1": "Apple",
		"B8FF61": "Apple",
		"BC3BAF": "Apple",
		"BC4CC4": "Apple",
		"BC52B7": "Apple",
		"BC6778": "Apple",
		"C01ADA": "Apple",
		"C0CECD": "Apple",
		"C81EE7": "Apple",
		"C82A14": "Apple",
		"C8334B": "Apple",
		"C869CD": "Apple",
		"C88550": "Apple",
		"C8B5B7": "Apple",
		"CC088D": "Apple",
		"CC29F5": "Apple",
		"D023DB": "Apple",
		"D49A20": "Apple",
		"D4F46F": "Apple",
		"D81D72": "Apple",
		"D89E3F": "Apple",
		"D8A25E": "Apple",
		"D8BB2C": "Apple",
		"DC0C5C": "Apple",
		"DC2B2A": "Apple",
		"DC2B61": "Apple",
		"DC86D8": "Apple",
		"DCA4CA": "Apple",
		"E05F45": "Apple",
		"E0ACCB": "Apple",
		"E0B52D": "Apple",
		"E0C767": "Apple",
		"E0F5C6": "Apple",
		"E0F847": "Apple",
		"E4C63D": "Apple",
		"E4CE8F": "Apple",
		"E80688": "Apple",
		"E89120": "Apple",
		"F02475": "Apple",
		"F04F7C": "Apple",
		"F0989D": "Apple",
		"F0B479": "Apple",
		"F0C1F1": "Apple",
		"F0D1A9": "Apple",
		"F0DBE2": "Apple",
		"F41BA1": "Apple",
		"F45C89": "Apple",
		"F4F15A": "Apple",
		"FC253F": "Apple",
		"FCFC48": "Apple",

		"000D3A": "Microsoft",
		"0015C5": "Dell",
		"0018FE": "HP",
		"001A4B": "HP",
		"001D0F": "TP-Link",
		"001E58": "D-Link",
		"0022FA": "Intel",
		"002314": "Intel",
		"002564": "Dell",
		"002590": "Super Micro",
		"0026C7": "Cisco",
		"00505E": "Cisco",
		"002618": "ASUS",
		"0050F2": "Microsoft",
		"00E04C": "Realtek",
		"080027": "VirtualBox",
		"0A0027": "VirtualBox",
		"18C04D": "Gree",
		"2491FB": "Amazon",
		"28EE52": "TP-Link",
		"40F520": "Google",
		"485D36": "Google",
		"50E14A": "Amazon",
		"5C5188": "HP",
		"6045CB": "ASUS",
		"68D79A": "Cisco",
		"6C72E7": "Intel",
		"744D28": "Samsung",
		"7825AD": "Samsung",
		"78E1AB": "HP",
		"8C85C1": "Samsung",
		"94B86D": "Samsung",
		"9C5C8E": "ASUS",
		"A0AFBD": "Intel",
		"A44CC8": "Intel",
		"AC220B": "ASUS",
		"B8AC6F": "Dell",
		"BC5C4C": "Elecom",
		"C0A0BB": "D-Link",
		"C4AC59": "Synology",
		"D067E5": "Dell",
		"D45D64": "ASUS",
		"E4BEED": "Netgear",
		"EC086B": "TP-Link",
		"F4EC38": "TP-Link",
		"F8A2D6": "TP-Link",
	}

	if vendor, ok := vendors[oui]; ok {
		return "[" + vendor + "]"
	}
	return ""
}
