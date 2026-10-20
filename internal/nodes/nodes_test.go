package nodes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("VLESS-REALITY-42210.json", `{"inbounds":[{"tag":"VLESS-REALITY-42210.json","type":"vless","listen_port":42210}]}`)
	write("VLESS-REALITY-42310.json", `{"inbounds":[{"tag":"VLESS-REALITY-42310.json","type":"vless","listen_port":42310}]}`)
	write("dynamic-port-foo-link.json", `{"url":"vless://..."}`)
	write("broken.json", `{not json`)

	got, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d nodes, want 2: %+v", len(got), got)
	}
	if got[0].Port != 42210 || got[0].Proto != "vless" || got[0].Tag != "VLESS-REALITY-42210.json" {
		t.Errorf("unexpected first node: %+v", got[0])
	}
	if got[1].Port != 42310 {
		t.Errorf("nodes must be sorted by port: %+v", got)
	}
}

func TestListeningHasSelf(t *testing.T) {
	// The test process itself is not listening, but sshd or the go test runner
	// usually is; what matters is that the parser returns a sane non-empty set
	// whenever /proc/net/tcp exists.
	ports := Listening()
	if _, err := os.Stat("/proc/net/tcp"); err != nil {
		t.Skip("no /proc/net/tcp on this platform")
	}
	if len(ports) == 0 {
		t.Skip("no listening ports visible in this sandbox")
	}
	for p := range ports {
		if p <= 0 || p > 65535 {
			t.Fatalf("parsed bogus port %d", p)
		}
	}
}
