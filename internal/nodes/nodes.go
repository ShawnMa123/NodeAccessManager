// Package nodes discovers the sing-box inbounds from the 233boy layout: one
// JSON file per node under /etc/sing-box/conf, each holding a single inbound.
package nodes

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Node struct {
	Tag   string // inbound tag, which is also what the Clash API reports
	Proto string // vless, vmess, trojan, hysteria2, ...
	Port  int
	File  string
}

type confFile struct {
	Inbounds []struct {
		Tag        string `json:"tag"`
		Type       string `json:"type"`
		Listen     string `json:"listen"`
		ListenPort int    `json:"listen_port"`
	} `json:"inbounds"`
}

// Discover reads every node config file and returns the inbounds that listen on
// a fixed port. Files without inbounds (233boy's link cache, or anything a user
// dropped in) are skipped rather than treated as nodes.
func Discover(dir string) ([]Node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Node
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cf confFile
		if err := json.Unmarshal(b, &cf); err != nil {
			continue
		}
		for _, in := range cf.Inbounds {
			if in.ListenPort == 0 {
				continue
			}
			tag := in.Tag
			if tag == "" {
				tag = strings.TrimSuffix(e.Name(), ".json")
			}
			out = append(out, Node{Tag: tag, Proto: in.Type, Port: in.ListenPort, File: e.Name()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Port < out[j].Port })
	return out, nil
}

// Listening returns the set of local TCP ports in LISTEN state, read straight
// from /proc so NAM never has to shell out.
func Listening() map[int]bool {
	ports := map[int]bool{}
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Scan() // header
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 4 || fields[3] != "0A" { // 0A == TCP_LISTEN
				continue
			}
			i := strings.LastIndexByte(fields[1], ':')
			if i < 0 {
				continue
			}
			if p, err := strconv.ParseInt(fields[1][i+1:], 16, 32); err == nil {
				ports[int(p)] = true
			}
		}
		fh.Close()
	}
	return ports
}
