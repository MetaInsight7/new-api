package model

import "testing"

func TestFormatUserLogsHidesIpWhenDisabled(t *testing.T) {
	logs := []*Log{{Ip: "203.0.113.10", ChannelName: "private-channel"}}

	formatUserLogs(logs, 0, false)

	if logs[0].Ip != "" {
		t.Fatalf("expected user log IP to be hidden, got %q", logs[0].Ip)
	}
	if logs[0].ChannelName != "" {
		t.Fatalf("expected admin-only channel name to be hidden, got %q", logs[0].ChannelName)
	}
}

func TestFormatUserLogsKeepsIpWhenEnabled(t *testing.T) {
	logs := []*Log{{Ip: "203.0.113.10"}}

	formatUserLogs(logs, 0, true)

	if logs[0].Ip != "203.0.113.10" {
		t.Fatalf("expected opted-in user log IP to be preserved, got %q", logs[0].Ip)
	}
}
