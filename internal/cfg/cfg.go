// Package cfg holds NAM's own settings and reads the sing-box server config,
// which is where the Clash API address and secret live.
package cfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// Config is NAM's own settings. Every field has a usable default, so the file
// is optional and only needs the values that differ from the defaults.
type Config struct {
	SingboxConfig    string `json:"singbox_config"`
	SingboxConfDir   string `json:"singbox_conf_dir"`
	Socket           string `json:"socket"`
	StateFile        string `json:"state_file"`
	PollSeconds      int    `json:"poll_seconds"`
	OnlineTTLSeconds int    `json:"online_ttl_seconds"`
	CooldownSeconds  int    `json:"cooldown_seconds"`
	DefaultLimit     int    `json:"default_limit"`
	IPv6Prefix       int    `json:"ipv6_prefix"`
}

func Default() Config {
	return Config{
		SingboxConfig:    "/etc/sing-box/config.json",
		SingboxConfDir:   "/etc/sing-box/conf",
		Socket:           "/run/nam.sock",
		StateFile:        "/var/lib/nam/state.json",
		PollSeconds:      2,
		OnlineTTLSeconds: 30,
		CooldownSeconds:  60,
		DefaultLimit:     5,
		IPv6Prefix:       64,
	}
}

// Path is where NAM looks for its settings file. NAM_CONFIG overrides it, which
// is mainly how the end-to-end test runs against a throwaway setup.
const DefaultPath = "/etc/nam/config.json"

func Path() string {
	if p := os.Getenv("NAM_CONFIG"); p != "" {
		return p
	}
	return DefaultPath
}

// Load reads Path over the defaults. A missing file is not an error.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// ClashAPI returns the address sing-box exposes the Clash API on, plus its
// secret. Empty address means the API is not configured.
func ClashAPI(singboxConfig string) (addr, secret string, err error) {
	b, err := os.ReadFile(singboxConfig)
	if err != nil {
		return "", "", err
	}
	var sc struct {
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
				Secret             string `json:"secret"`
			} `json:"clash_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(b, &sc); err != nil {
		return "", "", fmt.Errorf("%s: %w", singboxConfig, err)
	}
	return sc.Experimental.ClashAPI.ExternalController, sc.Experimental.ClashAPI.Secret, nil
}

// URL turns a controller address into a base URL, accepting either
// "127.0.0.1:9090" or "http://127.0.0.1:9090".
func URL(addr string) string {
	if strings.Contains(addr, "://") {
		return strings.TrimRight(addr, "/")
	}
	return "http://" + addr
}

// IsLoopback reports whether the controller is bound to this machine only.
// Anything else means the Clash API is reachable from the network, which needs
// a secret and is worth warning about.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://"))
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

// EnsureClashAPI adds experimental.clash_api to the sing-box config when it is
// missing, so sing-box can report which client IPs are actually connected. It
// is idempotent and only ever fills in the two fields it owns. The first run
// leaves a .nam.bak copy behind.
//
// Returns whether the file was changed.
func EnsureClashAPI(path, wantAddr, wantSecret string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	exp, _ := root["experimental"].(map[string]any)
	if exp == nil {
		exp = map[string]any{}
	}
	api, _ := exp["clash_api"].(map[string]any)
	if api == nil {
		api = map[string]any{}
	}
	changed := false
	if s, _ := api["external_controller"].(string); s == "" {
		api["external_controller"] = wantAddr
		changed = true
	}
	if s, _ := api["secret"].(string); s == "" {
		api["secret"] = wantSecret
		changed = true
	}
	if !changed {
		return false, nil
	}
	exp["clash_api"] = api
	root["experimental"] = exp

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return false, err
	}
	out = append(out, '\n')

	if _, err := os.Stat(path + ".nam.bak"); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path+".nam.bak", raw, 0o600); err != nil {
			return false, fmt.Errorf("backup: %w", err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".nam-*.json")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return false, err
	}
	return true, os.Rename(tmp.Name(), path)
}
