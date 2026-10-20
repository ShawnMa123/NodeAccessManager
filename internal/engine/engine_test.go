package engine

import (
	"testing"
	"time"
)

func mk(ip string, lastActive, start time.Time) *entry {
	return &entry{ip: ip, ipKey: ip, lastActive: lastActive, start: start}
}

func ipStrings(list []*entry) []string {
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.ip
	}
	return out
}

func TestPickVictimsByLeastRecentActivity(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	list := []*entry{
		mk("10.0.0.1", t0.Add(5*time.Second), t0),
		mk("10.0.0.2", t0.Add(-30*time.Second), t0), // stalest
		mk("10.0.0.3", t0, t0),
		mk("10.0.0.4", t0.Add(-10*time.Second), t0),
		mk("10.0.0.5", t0.Add(-20*time.Second), t0),
		mk("10.0.0.6", t0.Add(1*time.Second), t0),
	}

	got := ipStrings(pickVictims(list, 5))
	if len(got) != 1 || got[0] != "10.0.0.2" {
		t.Fatalf("limit 5 over 6 IPs: got %v, want [10.0.0.2]", got)
	}

	got = ipStrings(pickVictims(list, 3))
	want := []string{"10.0.0.2", "10.0.0.5", "10.0.0.4"}
	if len(got) != 3 {
		t.Fatalf("limit 3 over 6 IPs: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("limit 3 over 6 IPs: got %v, want %v", got, want)
		}
	}
}

func TestPickVictimsNoopWithinLimit(t *testing.T) {
	t0 := time.Unix(0, 0)
	list := []*entry{mk("203.0.113.11", t0, t0), mk("203.0.113.12", t0, t0)}
	if got := pickVictims(list, 5); got != nil {
		t.Fatalf("got %v, want nil", ipStrings(got))
	}
	if got := pickVictims(list, 2); got != nil {
		t.Fatalf("exactly at limit must not kick, got %v", ipStrings(got))
	}
}

func TestPickVictimsTieBreaksOnOldestSession(t *testing.T) {
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	list := []*entry{
		mk("10.0.0.9", t0, t0.Add(10*time.Second)),
		mk("10.0.0.8", t0, t0),
		mk("10.0.0.7", t0, t0.Add(5*time.Second)),
	}
	got := ipStrings(pickVictims(list, 1))
	if len(got) != 2 || got[0] != "10.0.0.8" || got[1] != "10.0.0.7" {
		t.Fatalf("got %v, want [10.0.0.8 10.0.0.7]", got)
	}
}
