// Package graph generates SVG time-series charts from nstat CSV files.
// No external dependencies — output is a single SVG file openable in any browser.
package graph

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	svgWidth     = 1400
	panelH       = 200
	padTop       = 25
	padBot       = 45
	padLeft      = 80
	padRight     = 30
	plotW        = svgWidth - padLeft - padRight // 1290
	plotH        = panelH - padTop - padBot      // 130
	headerH      = 45
	gridLines    = 5
)

type Point struct {
	T time.Time
	V float64
}

type Panel struct {
	Name  string
	Unit  string
	Warn  float64
	Crit  float64
	Data  []Point
}

type Options struct {
	Title  string
	Cutoff *time.Time // nil = all data
}

// Generate writes an SVG chart to outputPath from the given panels.
func Generate(panels []Panel, opts Options, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	totalH := headerH + len(panels)*panelH + 20

	fmt.Fprintf(f, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, svgWidth, totalH)
	fmt.Fprintln(f)
	fmt.Fprintf(f, `<rect width="%d" height="%d" fill="#1e1e2e"/>`, svgWidth, totalH)
	fmt.Fprintln(f)
	fmt.Fprintf(f, `<text x="%d" y="28" text-anchor="middle" font-family="monospace" `+
		`font-size="14" font-weight="bold" fill="#cdd6f4">%s</text>`,
		svgWidth/2, opts.Title)
	fmt.Fprintln(f)

	for i, p := range panels {
		yOff := headerH + i*panelH
		renderPanel(f, p, yOff, opts.Cutoff)
	}

	fmt.Fprintln(f, `</svg>`)
	return nil
}

func renderPanel(w io.Writer, p Panel, yOff int, cutoff *time.Time) {
	data := p.Data
	if cutoff != nil {
		var filtered []Point
		for _, pt := range data {
			if !pt.T.Before(*cutoff) {
				filtered = append(filtered, pt)
			}
		}
		data = filtered
	}

	// panel background
	fmt.Fprintf(w, `<rect x="0" y="%d" width="%d" height="%d" fill="#181825"/>`,
		yOff, svgWidth, panelH)
	fmt.Fprintln(w)

	// panel label
	fmt.Fprintf(w, `<text x="8" y="%d" font-family="monospace" font-size="11" fill="#cdd6f4">%s (%s)</text>`,
		yOff+padTop+8, p.Name, p.Unit)
	fmt.Fprintln(w)

	// plot area outline
	px, py := padLeft, yOff+padTop
	fmt.Fprintf(w, `<rect x="%d" y="%d" width="%d" height="%d" fill="#1e1e2e" stroke="#313244" stroke-width="0.5"/>`,
		px, py, plotW, plotH)
	fmt.Fprintln(w)

	if len(data) == 0 {
		fmt.Fprintf(w, `<text x="%d" y="%d" text-anchor="middle" font-family="monospace" font-size="10" fill="#6c7086">no data yet</text>`,
			padLeft+plotW/2, py+plotH/2+4)
		fmt.Fprintln(w)
		return
	}

	// compute ranges
	minT, maxT := data[0].T, data[0].T
	maxV := 0.0
	for _, pt := range data {
		if pt.T.Before(minT) {
			minT = pt.T
		}
		if pt.T.After(maxT) {
			maxT = pt.T
		}
		if pt.V > maxV {
			maxV = pt.V
		}
	}
	if maxV < p.Crit*1.2 {
		maxV = p.Crit * 1.2
	}
	if maxV == 0 {
		maxV = 1
	}
	tRange := maxT.Sub(minT).Seconds()
	if tRange == 0 {
		tRange = 1
	}

	scaleX := func(t time.Time) float64 {
		return float64(padLeft) + float64(plotW)*t.Sub(minT).Seconds()/tRange
	}
	scaleY := func(v float64) float64 {
		frac := v / maxV
		if frac > 1 {
			frac = 1
		}
		return float64(py+plotH) - float64(plotH)*frac
	}

	// grid lines + y-axis labels
	for i := 1; i <= gridLines; i++ {
		gv := maxV * float64(i) / float64(gridLines)
		gy := scaleY(gv)
		fmt.Fprintf(w, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#313244" stroke-width="0.5"/>`,
			padLeft, gy, padLeft+plotW, gy)
		fmt.Fprintln(w)
		fmt.Fprintf(w, `<text x="%d" y="%.1f" text-anchor="end" font-family="monospace" font-size="8" fill="#6c7086">%.1f</text>`,
			padLeft-3, gy+3, gv)
		fmt.Fprintln(w)
	}

	// warn line
	if p.Warn > 0 && p.Warn != p.Crit {
		wy := scaleY(p.Warn)
		fmt.Fprintf(w, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#f9e2af" stroke-width="0.8" stroke-dasharray="4,4" opacity="0.8"/>`,
			padLeft, wy, padLeft+plotW, wy)
		fmt.Fprintln(w)
		fmt.Fprintf(w, `<text x="%d" y="%.1f" font-family="monospace" font-size="8" fill="#f9e2af">warn %.0f</text>`,
			padLeft+plotW+2, wy+3, p.Warn)
		fmt.Fprintln(w)
	}
	// crit line
	if p.Crit > 0 {
		cy := scaleY(p.Crit)
		fmt.Fprintf(w, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#f38ba8" stroke-width="0.8" stroke-dasharray="4,4" opacity="0.8"/>`,
			padLeft, cy, padLeft+plotW, cy)
		fmt.Fprintln(w)
		fmt.Fprintf(w, `<text x="%d" y="%.1f" font-family="monospace" font-size="8" fill="#f38ba8">crit %.0f</text>`,
			padLeft+plotW+2, cy+3, p.Crit)
		fmt.Fprintln(w)
	}

	// fill polygon
	var fillPts strings.Builder
	fmt.Fprintf(&fillPts, "%.1f,%.1f ", scaleX(data[0].T), float64(py+plotH))
	for _, pt := range data {
		fmt.Fprintf(&fillPts, "%.1f,%.1f ", scaleX(pt.T), scaleY(pt.V))
	}
	fmt.Fprintf(&fillPts, "%.1f,%.1f", scaleX(data[len(data)-1].T), float64(py+plotH))
	fmt.Fprintf(w, `<polygon points="%s" fill="#89b4fa" opacity="0.12"/>`, fillPts.String())
	fmt.Fprintln(w)

	// data polyline
	var pts strings.Builder
	for _, pt := range data {
		fmt.Fprintf(&pts, "%.1f,%.1f ", scaleX(pt.T), scaleY(pt.V))
	}
	fmt.Fprintf(w, `<polyline points="%s" fill="none" stroke="#89b4fa" stroke-width="1.5" stroke-linejoin="round"/>`,
		strings.TrimSpace(pts.String()))
	fmt.Fprintln(w)

	// x-axis labels (up to 10 ticks)
	ticks := xTicks(minT, maxT, 10)
	timeFmt := xTimeFmt(maxT.Sub(minT))
	for _, t := range ticks {
		tx := scaleX(t)
		if tx < float64(padLeft) || tx > float64(padLeft+plotW) {
			continue
		}
		fmt.Fprintf(w, `<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="#45475a" stroke-width="0.5"/>`,
			tx, py+plotH, tx, py+plotH+4)
		fmt.Fprintln(w)
		label := t.Format(timeFmt)
		fmt.Fprintf(w, `<text x="%.1f" y="%d" text-anchor="middle" font-family="monospace" font-size="8" fill="#6c7086" transform="rotate(-30,%.1f,%d)">%s</text>`,
			tx, py+plotH+14, tx, py+plotH+14, label)
		fmt.Fprintln(w)
	}
}

func xTicks(minT, maxT time.Time, n int) []time.Time {
	if minT.Equal(maxT) {
		return []time.Time{minT}
	}
	d := maxT.Sub(minT) / time.Duration(n)
	var ticks []time.Time
	for i := 0; i <= n; i++ {
		ticks = append(ticks, minT.Add(time.Duration(i)*d))
	}
	return ticks
}

func xTimeFmt(d time.Duration) string {
	if d < 24*time.Hour {
		return "15:04"
	}
	return "01-02 15:04"
}

// LoadCSV reads a nstat CSV file and returns the points, optionally filtering
// to the last `hours` hours (0 = all).
func LoadCSV(path string, hours float64) ([]Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Read() // skip header

	var cutoff *time.Time
	if hours > 0 {
		t := time.Now().Add(-time.Duration(hours * float64(time.Hour)))
		cutoff = &t
	}

	var pts []Point
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		if len(rec) < 3 {
			continue
		}
		t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(rec[1]), time.Local)
		if err != nil {
			continue
		}
		var v float64
		fmt.Sscanf(strings.TrimSpace(rec[2]), "%f", &v)
		if cutoff != nil && t.Before(*cutoff) {
			continue
		}
		pts = append(pts, Point{T: t, V: v})
	}
	return pts, nil
}

// PanelDef describes a panel to render in the graph.
type PanelDef struct {
	Name    string
	CSVFile string
	Unit    string
	Warn    float64
	Crit    float64
}

var DefaultPanels = []PanelDef{
	{"RTT avg", "csv_rtt_avg.csv", "ms", 80, 200},
	{"Jitter", "csv_jitter.csv", "ms", 10, 30},
	{"Packet Loss", "csv_packet_loss.csv", "%", 1, 5},
	{"TCP Connect", "csv_tcp_connect.csv", "ms", 150, 150},
	{"TCP Loss", "csv_tcp_loss.csv", "%", 1, 5},
	{"DNS Resolve", "csv_dns.csv", "ms", 100, 500},
	{"DHCP Ping", "csv_dhcp.csv", "ms", 10, 50},
}

// BuildPanels loads CSV data for each panel definition from the given data directory.
func BuildPanels(defs []PanelDef, dataDir string, hours float64) []Panel {
	panels := make([]Panel, 0, len(defs))
	for _, d := range defs {
		path := filepath.Join(dataDir, d.CSVFile)
		pts, _ := LoadCSV(path, hours)
		panels = append(panels, Panel{
			Name: d.Name,
			Unit: d.Unit,
			Warn: d.Warn,
			Crit: d.Crit,
			Data: pts,
		})
	}
	return panels
}

