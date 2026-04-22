# nstat

A lightweight network connection reliability monitor that runs as a background daemon and continuously tracks multiple dimensions of your internet quality.

## What it monitors

| Dimension | Description | Good / Warn / Crit |
|---|---|---|
| RTT (avg) | Rolling ICMP round-trip time to 8.8.8.8 | < 80 ms / 80–200 ms / > 200 ms |
| Jitter | Standard deviation of RTT (same window) | < 10 ms / 10–30 ms / > 30 ms |
| Packet loss | % of ICMP pings that got no reply | < 1% / 1–5% / > 5% |
| TCP connect | Time for TCP handshake to 8.8.8.8:53 | < 150 ms / — / failed |
| TCP loss | % of TCP connection attempts that failed | < 1% / 1–5% / > 5% |
| DNS | Resolution time for google.com via your DNS server | < 100 ms / 100–500 ms / failed |
| DHCP (gateway) | ICMP ping to your default gateway (LAN health) | < 10 ms / 10–50 ms / failed |
| Outages/1h | Distinct outage events (≥ 3 consecutive losses) in the last hour | 0 / 1–2 / ≥ 3 |

## Installation

### Download a pre-built binary

Go to the [Releases page](../../releases) and download the binary for your platform:

| Platform | File |
|---|---|
| Linux x86-64 | `nstat-linux-amd64` |
| Linux ARM64 (Raspberry Pi, etc.) | `nstat-linux-arm64` |
| macOS Intel | `nstat-darwin-amd64` |
| macOS Apple Silicon (M1/M2/M3) | `nstat-darwin-arm64` |
| Windows x86-64 | `nstat-windows-amd64.exe` |

**Linux / macOS:**
```sh
# replace <version> and <platform> with the values for your system
curl -L https://github.com/<owner>/nstat/releases/download/<version>/nstat-<platform> -o nstat
chmod +x nstat
sudo mv nstat /usr/local/bin/
```

**Windows:** download the `.exe` and place it somewhere on your `PATH`.

### Build from source

Requires Go 1.23+.

```sh
git clone https://github.com/<owner>/nstat.git
cd nstat
make build          # build for the current OS/arch
make install        # install to ~/.local/bin/nstat  (Linux/macOS)
```

To build release binaries for all platforms at once:

```sh
make dist           # output in ./dist/
```

## Usage

```
nstat start [--interval N] [--window N]
nstat stop
nstat status
nstat log
nstat graph [--hours N]
nstat -h
```

### Commands

| Command | Description |
|---|---|
| `nstat start` | Start the background daemon |
| `nstat stop` | Gracefully stop the daemon |
| `nstat status` | Print current network health with colour-coded scores |
| `nstat log` | Show the last 40 lines of the live log |
| `nstat graph` | Generate an SVG chart and open it in the default viewer |
| `nstat graph --hours N` | Limit the chart to the last N hours |

### Start options

| Option | Default | Description |
|---|---|---|
| `--interval N` | 5 | Seconds between ICMP pings |
| `--window N` | 60 | Number of pings used for the rolling RTT/jitter average |

Example: 1-second pings with a 2-minute rolling average:
```sh
nstat start --interval 1 --window 120
```

## Platform notes

### Linux

Full support. Unprivileged ICMP pings work out of the box on most distributions
(the kernel allows them via `net.ipv4.ping_group_range`). If pings fail, run:

```sh
sudo sysctl -w net.ipv4.ping_group_range="0 2147483647"
```

### macOS

Full support. ICMP requires root or `NET_RAW` entitlements:

```sh
sudo nstat start
```

### Windows

Functional but with limitations:
- ICMP requires administrator privileges — run from an elevated Command Prompt or PowerShell.
- The daemon runs as a hidden background process (not a Windows Service); use `nstat stop` to terminate it.
- Gateway and DNS detection rely on `ipconfig` output.

## Run at startup

### Linux (systemd)

Create the service file directory and open the unit file in an editor:

```sh
mkdir -p ~/.config/systemd/user
$EDITOR ~/.config/systemd/user/nstat.service
```

Paste the following content:

```ini
[Unit]
Description=nstat network monitor
After=network-online.target

[Service]
Type=forking
ExecStart=%h/.local/bin/nstat start
PIDFile=%h/.local/share/nstat/nstat.pid
Restart=on-failure

[Install]
WantedBy=default.target
```

Then reload systemd and enable the service:

```sh
systemctl --user daemon-reload
systemctl --user enable --now nstat
```

To keep the daemon running at boot even without an active login session (useful on servers or headless machines), enable lingering for your user:

```sh
sudo loginctl enable-linger $USER
```

Useful commands:

```sh
systemctl --user status nstat    # check status
systemctl --user stop nstat      # stop the daemon
journalctl --user -u nstat       # view systemd logs
```

### macOS (launchd)

Create a LaunchAgent that runs `nstat start` when you log in:

```sh
cat > ~/Library/LaunchAgents/com.github.nstat.plist << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.github.nstat</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/nstat</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
EOF

launchctl load ~/Library/LaunchAgents/com.github.nstat.plist
```

> **Note:** macOS requires root for ICMP. If you run `sudo nstat start`, use a LaunchDaemon (`/Library/LaunchDaemons/`) instead, owned by root, with `<key>UserName</key><string>root</string>` added to the plist.

To unload the agent:

```sh
launchctl unload ~/Library/LaunchAgents/com.github.nstat.plist
```

### Windows (Task Scheduler)

Run the following from an elevated (administrator) PowerShell or Command Prompt:

```powershell
schtasks /Create /TN "nstat" /TR "\"C:\path\to\nstat.exe\" start" /SC ONLOGON /RL HIGHEST /F
```

Replace `C:\path\to\nstat.exe` with the actual path to the binary (e.g. `C:\Tools\nstat.exe`).

To remove the task:

```powershell
schtasks /Delete /TN "nstat" /F
```

## Data storage

| Platform | Data directory |
|---|---|
| Linux | `~/.local/share/nstat/` |
| macOS | `~/Library/Application Support/nstat/` |
| Windows | `%APPDATA%\nstat\` |

Files written:

| File | Description |
|---|---|
| `nstat.log` | Live event log (rotated every 24 h → `.1`, `.2`, `.3`) |
| `nstat.state.json` | Current snapshot read by `nstat status` |
| `nstat.pid` | PID of the running daemon |
| `nstat_graph.svg` | Last generated chart |
| `csv_*.csv` | Per-dimension time-series data (never rotated) |

## Permissions

nstat uses raw ICMP sockets as a fallback when the kernel does not allow
unprivileged UDP-based ICMP. Summary:

| Scenario | Privileges needed |
|---|---|
| Linux with `ping_group_range` set | None |
| Linux without `ping_group_range` | `CAP_NET_RAW` or run as root |
| macOS | Root or `NET_RAW` entitlement |
| Windows | Administrator |
