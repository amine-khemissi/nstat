package state

import (
	"encoding/json"
	"os"
	"time"
)

type State struct {
	Timestamp    string  `json:"timestamp"`
	Epoch        int64   `json:"epoch"`
	PingInterval int     `json:"ping_interval"`
	LANInterval  int     `json:"lan_interval"`
	RTTWindow    int     `json:"rtt_window"`
	RTTCurrent   float64 `json:"rtt_current"`
	RTTAvg       float64 `json:"rtt_avg"`
	RTTJitter    float64 `json:"rtt_jitter"`
	LossTotal    int     `json:"loss_total"`
	PingsTotal   int     `json:"pings_total"`
	LossPct      float64 `json:"loss_pct"`
	TCPLastMs    float64 `json:"tcp_last_ms"`
	TCPLastOK    bool    `json:"tcp_last_ok"`
	TCPTotal     int     `json:"tcp_total"`
	TCPFail      int     `json:"tcp_fail"`
	TCPLossPct   float64 `json:"tcp_loss_pct"`
	DNSServer    string  `json:"dns_server"`
	DNSLastMs    float64 `json:"dns_last_ms"`
	DNSLastOK    bool    `json:"dns_last_ok"`
	DHCPServer   string  `json:"dhcp_server"`
	DHCPLastMs   float64 `json:"dhcp_last_ms"`
	DHCPLastOK   bool    `json:"dhcp_last_ok"`
	InOutage     bool    `json:"in_outage"`
	OutageCount  int     `json:"outage_count"`
	RecentOutage []int64 `json:"recent_outage_ts"`
	SessionStart int64   `json:"session_start"`
}

func Write(path string, s *State) error {
	s.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	s.Epoch = time.Now().Unix()

	tmp := path + ".tmp"
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Read(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s State
	return &s, json.Unmarshal(b, &s)
}
