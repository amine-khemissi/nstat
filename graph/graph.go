// Package graph generates an interactive HTML dashboard from nstat CSV files.
// Uses Plotly.js for zoomable, synchronized time-series charts.
package graph

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Point struct {
	T time.Time
	V float64
}

type Panel struct {
	Name string
	Unit string
	Warn float64
	Crit float64
	Data []Point
}

type Options struct {
	Title  string
	Cutoff *time.Time
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
	{"TCP Retrans", "csv_tcp_retrans.csv", "%", 2, 5},
	{"TCP Errors", "csv_tcp_errors.csv", "count", 1, 10},
	{"MTU", "csv_mtu.csv", "bytes", 1400, 1200},
	{"DNS Resolve", "csv_dns.csv", "ms", 100, 500},
	{"DHCP Ping", "csv_dhcp.csv", "ms", 10, 50},
}

// Generate writes an interactive HTML dashboard to outputPath.
func Generate(panels []Panel, opts Options, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Filter panels with data
	var activePanels []Panel
	for _, p := range panels {
		if len(p.Data) > 0 {
			activePanels = append(activePanels, p)
		}
	}

	if len(activePanels) == 0 {
		return fmt.Errorf("no data to graph")
	}

	// Write HTML header
	fmt.Fprint(f, htmlHeader(opts.Title))

	// Write data as JSON
	fmt.Fprintln(f, "<script>")
	fmt.Fprintln(f, "const panelData = [")
	for i, p := range activePanels {
		writeJSONPanel(f, p, i == len(activePanels)-1)
	}
	fmt.Fprintln(f, "];")
	fmt.Fprintln(f, "</script>")

	// Write Plotly rendering code
	fmt.Fprint(f, htmlPlotlyCode(len(activePanels)))

	// Write HTML footer
	fmt.Fprint(f, htmlFooter())

	return nil
}

func writeJSONPanel(f *os.File, p Panel, isLast bool) {
	// Sample data if too large
	data := sampleData(p.Data, 3000)

	fmt.Fprintf(f, `  {name: %q, unit: %q, warn: %v, crit: %v, x: [`, p.Name, p.Unit, p.Warn, p.Crit)

	// Write timestamps
	for i, pt := range data {
		if i > 0 {
			fmt.Fprint(f, ",")
		}
		fmt.Fprintf(f, `"%s"`, pt.T.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprint(f, "], y: [")

	// Write values
	for i, pt := range data {
		if i > 0 {
			fmt.Fprint(f, ",")
		}
		fmt.Fprintf(f, "%.4f", pt.V)
	}
	fmt.Fprint(f, "]}")

	if !isLast {
		fmt.Fprintln(f, ",")
	} else {
		fmt.Fprintln(f, "")
	}
}

func htmlHeader(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>%s</title>
  <script src="https://cdn.plot.ly/plotly-2.27.0.min.js"></script>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, monospace;
      background: #1e1e2e;
      color: #cdd6f4;
      padding: 16px;
    }
    h1 {
      text-align: center;
      font-size: 1.4em;
      margin-bottom: 8px;
      color: #89b4fa;
    }
    .controls {
      text-align: center;
      margin-bottom: 12px;
    }
    .controls button {
      background: #313244;
      border: 1px solid #45475a;
      color: #cdd6f4;
      padding: 6px 14px;
      margin: 0 4px;
      border-radius: 4px;
      cursor: pointer;
      font-size: 0.9em;
    }
    .controls button:hover { background: #45475a; }
    .controls button.active { background: #89b4fa; color: #1e1e2e; }
    #dashboard { width: 100%%; }
    .panel {
      background: #181825;
      border-radius: 6px;
      margin-bottom: 8px;
      padding: 8px;
    }
    .panel-title {
      font-size: 0.85em;
      color: #a6adc8;
      margin-bottom: 4px;
      padding-left: 8px;
    }
    .plot { width: 100%%; height: 180px; }
    .footer {
      text-align: center;
      font-size: 0.75em;
      color: #6c7086;
      margin-top: 16px;
    }
  </style>
</head>
<body>
  <h1>%s</h1>
  <div class="controls">
    <button onclick="setRange(1)">1h</button>
    <button onclick="setRange(6)">6h</button>
    <button onclick="setRange(24)">24h</button>
    <button onclick="setRange(72)">3d</button>
    <button onclick="setRange(168)">7d</button>
    <button onclick="setRange(0)" class="active">All</button>
    <button onclick="resetZoom()">Reset Zoom</button>
  </div>
  <div id="dashboard"></div>
`, title, title)
}

func htmlPlotlyCode(numPanels int) string {
	return `<script>
const plots = [];
const colors = {
  line: '#89b4fa',
  fill: 'rgba(137, 180, 250, 0.15)',
  warn: '#f9e2af',
  crit: '#f38ba8',
  grid: '#313244',
  bg: '#181825',
  paper: '#1e1e2e',
  text: '#cdd6f4'
};

function createPlots() {
  const dashboard = document.getElementById('dashboard');

  panelData.forEach((panel, idx) => {
    const div = document.createElement('div');
    div.className = 'panel';
    div.innerHTML = '<div class="panel-title">' + panel.name + ' (' + panel.unit + ')</div><div id="plot' + idx + '" class="plot"></div>';
    dashboard.appendChild(div);

    const trace = {
      x: panel.x,
      y: panel.y,
      type: 'scatter',
      mode: 'lines',
      name: panel.name,
      line: { color: colors.line, width: 1.5 },
      fill: 'tozeroy',
      fillcolor: colors.fill,
      hovertemplate: '%{y:.2f} ' + panel.unit + '<extra>%{x}</extra>'
    };

    const shapes = [];

    // Warning threshold line
    if (panel.warn > 0 && panel.warn !== panel.crit) {
      shapes.push({
        type: 'line',
        xref: 'paper', x0: 0, x1: 1,
        yref: 'y', y0: panel.warn, y1: panel.warn,
        line: { color: colors.warn, width: 1, dash: 'dash' }
      });
    }

    // Critical threshold line
    if (panel.crit > 0) {
      shapes.push({
        type: 'line',
        xref: 'paper', x0: 0, x1: 1,
        yref: 'y', y0: panel.crit, y1: panel.crit,
        line: { color: colors.crit, width: 1, dash: 'dash' }
      });
    }

    const layout = {
      margin: { l: 60, r: 20, t: 10, b: 30 },
      paper_bgcolor: colors.paper,
      plot_bgcolor: colors.bg,
      font: { color: colors.text, size: 10 },
      xaxis: {
        type: 'date',
        gridcolor: colors.grid,
        linecolor: colors.grid,
        tickformat: '%H:%M',
        hoverformat: '%Y-%m-%d %H:%M:%S'
      },
      yaxis: {
        gridcolor: colors.grid,
        linecolor: colors.grid,
        title: { text: panel.unit, standoff: 5 },
        rangemode: 'tozero'
      },
      shapes: shapes,
      hovermode: 'x unified',
      showlegend: false
    };

    const config = {
      responsive: true,
      displayModeBar: true,
      modeBarButtonsToRemove: ['lasso2d', 'select2d', 'autoScale2d'],
      displaylogo: false
    };

    Plotly.newPlot('plot' + idx, [trace], layout, config);
    plots.push(document.getElementById('plot' + idx));
  });

  // Sync zoom/pan across all plots
  plots.forEach((plot, idx) => {
    plot.on('plotly_relayout', (eventData) => {
      if (eventData['xaxis.autorange'] || eventData['xaxis.range[0]'] !== undefined) {
        const xRange = eventData['xaxis.range[0]'] ?
          [eventData['xaxis.range[0]'], eventData['xaxis.range[1]']] : null;

        plots.forEach((otherPlot, otherIdx) => {
          if (otherIdx !== idx) {
            const update = xRange ?
              { 'xaxis.range': xRange } :
              { 'xaxis.autorange': true };
            Plotly.relayout(otherPlot, update);
          }
        });
      }
    });
  });
}

function setRange(hours) {
  // Update button states
  document.querySelectorAll('.controls button').forEach(b => b.classList.remove('active'));
  event.target.classList.add('active');

  if (hours === 0) {
    // Show all data
    plots.forEach(plot => {
      Plotly.relayout(plot, { 'xaxis.autorange': true });
    });
  } else {
    const now = new Date();
    const start = new Date(now.getTime() - hours * 60 * 60 * 1000);
    const range = [start.toISOString(), now.toISOString()];

    plots.forEach(plot => {
      Plotly.relayout(plot, { 'xaxis.range': range });
    });
  }
}

function resetZoom() {
  plots.forEach(plot => {
    Plotly.relayout(plot, {
      'xaxis.autorange': true,
      'yaxis.autorange': true
    });
  });
}

createPlots();
</script>
`
}

func htmlFooter() string {
	return fmt.Sprintf(`  <div class="footer">
    Generated by nstat · %s
  </div>
</body>
</html>
`, time.Now().Format("2006-01-02 15:04:05"))
}

// sampleData reduces data points using LTTB algorithm for visual fidelity.
func sampleData(data []Point, maxPts int) []Point {
	if len(data) <= maxPts {
		return data
	}

	result := make([]Point, 0, maxPts)
	result = append(result, data[0])

	bucketSize := float64(len(data)-2) / float64(maxPts-2)

	for i := 1; i < maxPts-1; i++ {
		bucketStart := int(float64(i-1)*bucketSize) + 1
		bucketEnd := int(float64(i)*bucketSize) + 1
		if bucketEnd > len(data)-1 {
			bucketEnd = len(data) - 1
		}

		nextBucketStart := bucketEnd
		nextBucketEnd := int(float64(i+1)*bucketSize) + 1
		if nextBucketEnd > len(data) {
			nextBucketEnd = len(data)
		}

		var avgT, avgV float64
		for j := nextBucketStart; j < nextBucketEnd; j++ {
			avgT += float64(data[j].T.Unix())
			avgV += data[j].V
		}
		nextCount := float64(nextBucketEnd - nextBucketStart)
		if nextCount > 0 {
			avgT /= nextCount
			avgV /= nextCount
		}

		prevPt := result[len(result)-1]
		maxArea := -1.0
		var maxPt Point
		for j := bucketStart; j < bucketEnd; j++ {
			pt := data[j]
			area := triangleArea(
				float64(prevPt.T.Unix()), prevPt.V,
				float64(pt.T.Unix()), pt.V,
				avgT, avgV,
			)
			if area > maxArea {
				maxArea = area
				maxPt = pt
			}
		}
		if maxArea >= 0 {
			result = append(result, maxPt)
		}
	}

	result = append(result, data[len(data)-1])
	return result
}

func triangleArea(x1, y1, x2, y2, x3, y3 float64) float64 {
	area := (x1*(y2-y3) + x2*(y3-y1) + x3*(y1-y2)) / 2
	if area < 0 {
		area = -area
	}
	return area
}

// LoadCSV reads a nstat CSV file and returns the points.
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
