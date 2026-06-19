//go:build linux

package daemon

import (
	"testing"
	"time"
)

func TestParseDhclientLeases_LatestMatching(t *testing.T) {
	content := `
lease {
  interface "enp0s31f6";
  option dhcp-lease-time 3600;
  option dhcp-server-identifier 192.168.100.254;
  expire 4 2026/06/19 18:30:00;
}
lease {
  interface "enp0s31f6";
  option dhcp-lease-time 86400;
  option dhcp-server-identifier 192.168.100.1;
  expire 5 2026/06/20 12:30:19;
}
`
	server, expiry, lease, ok := parseDhclientLeases(content, "enp0s31f6")
	if !ok {
		t.Fatal("expected a lease")
	}
	want, _ := time.ParseInLocation("2006/01/02 15:04:05", "2026/06/20 12:30:19", time.UTC)
	if expiry != want.Unix() {
		t.Errorf("expiry = %d, want %d (latest block)", expiry, want.Unix())
	}
	if server != "192.168.100.1" {
		t.Errorf("server = %q, want 192.168.100.1", server)
	}
	if lease != 86400 {
		t.Errorf("lease = %d, want 86400", lease)
	}
}

func TestParseDhclientLeases_IfaceFilter(t *testing.T) {
	content := `lease {
  interface "eth9";
  option dhcp-server-identifier 10.0.0.1;
  option dhcp-lease-time 600;
  expire 5 2030/01/01 00:00:00;
}`
	if _, _, _, ok := parseDhclientLeases(content, "enp0s31f6"); ok {
		t.Error("should not match a different interface")
	}
}

func TestParseNetworkdLease(t *testing.T) {
	content := "# This is private data. Do not parse.\n" +
		"ADDRESS=192.168.100.69\nROUTER=192.168.100.1\n" +
		"SERVER_ADDRESS=192.168.100.1\nLIFETIME=86400\n"
	mtime := time.Unix(1_000_000, 0)
	server, expiry, lease, ok := parseNetworkdLease(content, mtime)
	if !ok {
		t.Fatal("expected ok")
	}
	if server != "192.168.100.1" || lease != 86400 {
		t.Errorf("server/lease = %q/%d", server, lease)
	}
	if expiry != mtime.Unix()+86400 {
		t.Errorf("expiry = %d, want %d (mtime+LIFETIME)", expiry, mtime.Unix()+86400)
	}
}
