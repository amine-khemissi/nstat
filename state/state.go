package state

import (
	"encoding/json"
	"os"
	"time"
)

// TCPTargetState holds per-target TCP statistics.
type TCPTargetState struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	LastMs       float64 `json:"last_ms"`
	LastOK       bool   `json:"last_ok"`
	LastReason   string `json:"last_reason"` // timeout, refused, reset, dns, other
	Total        int    `json:"total"`
	Fail         int    `json:"fail"`
	LossPct      float64 `json:"loss_pct"`
	TimeoutCount int    `json:"timeout_count"`
	RefusedCount int    `json:"refused_count"`
	ResetCount   int    `json:"reset_count"`
	OtherCount   int    `json:"other_count"`
}

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
	TCPLastReason string `json:"tcp_last_reason,omitempty"` // failure reason for primary target

	// Per-target TCP stats (multiple targets for granularity)
	TCPTargets []TCPTargetState `json:"tcp_targets,omitempty"`

	// TCP failure breakdown (aggregated across all targets)
	TCPTimeoutCount int `json:"tcp_timeout_count"`
	TCPRefusedCount int `json:"tcp_refused_count"`
	TCPResetCount   int `json:"tcp_reset_count"`
	TCPOtherCount   int `json:"tcp_other_count"`

	// MTU probe results
	MTUDetected   int     `json:"mtu_detected,omitempty"`
	MTULastMs     float64 `json:"mtu_last_ms,omitempty"`
	MTUHasIssues  bool    `json:"mtu_has_issues,omitempty"`
	MTUFailedSizes []int  `json:"mtu_failed_sizes,omitempty"`

	// Kernel TCP stats
	KernelRetransPct    float64 `json:"kernel_retrans_pct,omitempty"`
	KernelDeltaRetrans  int64   `json:"kernel_delta_retrans,omitempty"`
	KernelDeltaOutSegs  int64   `json:"kernel_delta_out_segs,omitempty"`
	KernelDeltaInErrs   int64   `json:"kernel_delta_in_errs,omitempty"`
	KernelDeltaResets   int64   `json:"kernel_delta_resets,omitempty"`
	KernelCurrEstab     int64   `json:"kernel_curr_estab,omitempty"`

	// TCP tuning (sysctl values)
	TCPSynRetries      int  `json:"tcp_syn_retries,omitempty"`
	TCPRetries2        int  `json:"tcp_retries2,omitempty"`
	TCPKeepaliveTime   int  `json:"tcp_keepalive_time,omitempty"`
	TCPKeepaliveIntvl  int  `json:"tcp_keepalive_intvl,omitempty"`
	TCPKeepaliveProbes int  `json:"tcp_keepalive_probes,omitempty"`
	TCPFinTimeout      int  `json:"tcp_fin_timeout,omitempty"`
	TCPFastFail        bool `json:"tcp_fast_fail,omitempty"`

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
