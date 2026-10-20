// Package engine is the part that decides and acts: poll sing-box, group the
// live connections per node and source IP, and drop the stalest IP when a node
// has more IPs online than its limit.
package engine

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/ShawnMa123/NodeAccessManager/internal/clash"
	"github.com/ShawnMa123/NodeAccessManager/internal/ipkey"
	"github.com/ShawnMa123/NodeAccessManager/internal/nodes"
	"github.com/ShawnMa123/NodeAccessManager/internal/state"
)

type Options struct {
	PollInterval time.Duration
	OnlineTTL    time.Duration
	Cooldown     time.Duration
	DefaultLimit int
	IPv6Prefix   int
}

type Event struct {
	At   time.Time `json:"at"`
	Kind string    `json:"kind"` // kick | allow | limit | api
	Node string    `json:"node"`
	IP   string    `json:"ip"`
	Note string    `json:"note"`
}

type IPView struct {
	IP          string `json:"ip"`
	Conns       int    `json:"conns"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
	ActiveAgo   int64  `json:"active_ago_seconds"`
	Cooling     bool   `json:"cooling"`
	Whitelisted bool   `json:"whitelisted"`
}

type NodeView struct {
	Tag       string   `json:"tag"`
	Proto     string   `json:"proto"`
	Port      int      `json:"port"`
	Listening bool     `json:"listening"`
	Limit     int      `json:"limit"`
	Online    int      `json:"online"`
	IPs       []IPView `json:"ips"`
}

type Snapshot struct {
	Now          time.Time  `json:"now"`
	Started      time.Time  `json:"started"`
	APIOK        bool       `json:"api_ok"`
	APIErr       string     `json:"api_err"`
	LastPoll     time.Time  `json:"last_poll"`
	DefaultLimit int        `json:"default_limit"`
	Whitelist    []string   `json:"whitelist"`
	Nodes        []NodeView `json:"nodes"`
	Events       []Event    `json:"events"`
}

type entry struct {
	node       string
	ipKey      string
	ip         string
	conns      map[string]struct{}
	up, down   int64
	start      time.Time
	lastSeen   time.Time
	lastActive time.Time
}

type Engine struct {
	mu   sync.Mutex
	opts Options
	api  *clash.Client
	st   *state.State
	list func() []nodes.Node

	entries  map[string]*entry
	connSeen map[string]int64
	events   []Event

	started  time.Time
	lastPoll time.Time
	apiOK    bool
	apiErr   string

	now func() time.Time
}

func New(opts Options, api *clash.Client, st *state.State, list func() []nodes.Node) *Engine {
	return &Engine{
		opts:     opts,
		api:      api,
		st:       st,
		list:     list,
		entries:  map[string]*entry{},
		connSeen: map[string]int64{},
		started:  time.Now(),
		now:      time.Now,
	}
}

func (e *Engine) SetAPI(api *clash.Client) {
	e.mu.Lock()
	e.api = api
	e.apiErr = ""
	e.mu.Unlock()
}

func (e *Engine) Run(ctx context.Context) {
	t := time.NewTicker(e.opts.PollInterval)
	defer t.Stop()
	e.Poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.Poll(ctx)
		}
	}
}

func key(node, ipKey string) string { return node + "\x00" + ipKey }

// Poll refreshes the picture from sing-box and enforces the limits once.
func (e *Engine) Poll(ctx context.Context) {
	conns, err := e.api.Connections(ctx)

	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.now()
	e.lastPoll = now
	if err != nil {
		if e.apiOK || e.apiErr != err.Error() {
			e.event(now, "api", "", "", "无法连接 Clash API: "+err.Error())
		}
		e.apiOK = false
		e.apiErr = err.Error()
		return
	}
	if !e.apiOK && e.apiErr != "" {
		e.event(now, "api", "", "", "Clash API 已恢复")
	}
	e.apiOK = true
	e.apiErr = ""

	for _, en := range e.entries {
		en.conns = map[string]struct{}{}
		en.up, en.down = 0, 0
	}

	seen := make(map[string]int64, len(conns))
	for _, c := range conns {
		ip := c.Metadata.SourceIP
		if ip == "" {
			continue
		}
		tag := c.NodeTag()
		k := key(tag, ipkey.Key(ip, e.opts.IPv6Prefix))
		en := e.entries[k]
		if en == nil {
			en = &entry{
				node:       tag,
				ipKey:      ipkey.Key(ip, e.opts.IPv6Prefix),
				ip:         ip,
				conns:      map[string]struct{}{},
				lastActive: now,
			}
			e.entries[k] = en
		}
		en.conns[c.ID] = struct{}{}
		en.up += c.Upload
		en.down += c.Download
		en.lastSeen = now
		if en.start.IsZero() || c.Start.Before(en.start) {
			en.start = c.Start
		}
		total := c.Upload + c.Download
		if prev, ok := e.connSeen[c.ID]; !ok || prev != total {
			en.lastActive = now
		}
		seen[c.ID] = total
	}
	e.connSeen = seen

	for k, en := range e.entries {
		if len(en.conns) == 0 && now.Sub(en.lastSeen) > e.opts.OnlineTTL {
			delete(e.entries, k)
		}
	}

	e.st.PruneCooldowns(now)
	e.enforce(ctx, now)
}

// enforce walks every node that has traffic and drops the stalest IPs above the
// node's limit. IPs inside their cooldown window are killed on sight so a
// kicked client cannot immediately come back and displace someone else.
func (e *Engine) enforce(ctx context.Context, now time.Time) {
	online := map[string][]*entry{}
	for _, en := range e.entries {
		if e.whitelisted(en.ip) {
			continue
		}
		if until, cooling := e.st.Cooldown(key(en.node, en.ipKey)); cooling && until.After(now) {
			if n := e.drop(ctx, en); n > 0 {
				e.event(now, "kick", en.node, en.ip, fmt.Sprintf("冷却期内重新连入，断开 %d 条", n))
			}
			continue
		}
		// Only IPs with a live connection occupy a slot. An entry kept around for
		// the TTL after it went quiet must not push a real user out.
		if len(en.conns) == 0 {
			continue
		}
		online[en.node] = append(online[en.node], en)
	}

	for node, list := range online {
		limit := e.limitFor(node)
		victims := pickVictims(list, limit)
		if len(victims) == 0 {
			continue
		}
		for _, en := range victims {
			n := e.drop(ctx, en)
			if n == 0 {
				// Already gone between the poll and the kick; nothing to report.
				delete(e.entries, key(en.node, en.ipKey))
				continue
			}
			e.st.SetCooldown(key(en.node, en.ipKey), now.Add(e.opts.Cooldown))
			e.event(now, "kick", en.node, en.ip, fmt.Sprintf("超出上限 %d，断开 %d 条连接，冷却 %s",
				limit, n, e.opts.Cooldown))
		}
	}
}

// pickVictims returns the entries to drop so that at most limit remain, least
// recently used first. Ties fall back to the oldest session, then to the
// address, so the order is stable across polls.
func pickVictims(list []*entry, limit int) []*entry {
	if limit <= 0 || len(list) <= limit {
		return nil
	}
	sorted := append([]*entry(nil), list...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if !a.lastActive.Equal(b.lastActive) {
			return a.lastActive.Before(b.lastActive)
		}
		if !a.start.Equal(b.start) {
			return a.start.Before(b.start)
		}
		return a.ip < b.ip
	})
	return sorted[:len(sorted)-limit]
}

// drop closes every live connection of one entry and reports how many the API
// accepted. Callers decide whether it is worth an event.
func (e *Engine) drop(ctx context.Context, en *entry) int {
	n := 0
	for id := range en.conns {
		if err := e.api.CloseConn(ctx, id); err == nil {
			n++
		}
	}
	return n
}

func (e *Engine) limitFor(node string) int {
	if v, ok := e.st.Limit(node); ok {
		return v
	}
	return e.opts.DefaultLimit
}

func (e *Engine) whitelisted(ip string) bool {
	for _, w := range e.st.WhitelistCopy() {
		if ipkey.Match(w, ip) {
			return true
		}
	}
	return false
}

func (e *Engine) event(at time.Time, kind, node, ip, note string) {
	e.events = append(e.events, Event{At: at, Kind: kind, Node: node, IP: ip, Note: note})
	if len(e.events) > 200 {
		e.events = e.events[len(e.events)-200:]
	}
	// Also going to stderr means the history survives a restart in journald.
	line := fmt.Sprintf("%s %s", kind, note)
	if node != "" {
		line += " [" + node + "]"
	}
	if ip != "" {
		line += " " + ip
	}
	fmt.Fprintln(os.Stderr, "nam: "+line)
}

// Status builds the read-only view the CLI renders.
func (e *Engine) Status() Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.now()
	snap := Snapshot{
		Now:          now,
		Started:      e.started,
		APIOK:        e.apiOK,
		APIErr:       e.apiErr,
		LastPoll:     e.lastPoll,
		DefaultLimit: e.opts.DefaultLimit,
		Whitelist:    e.st.WhitelistCopy(),
	}
	for i := len(e.events) - 1; i >= 0; i-- {
		snap.Events = append(snap.Events, e.events[i])
		if len(snap.Events) >= 30 {
			break
		}
	}

	byNode := map[string][]*entry{}
	for _, en := range e.entries {
		byNode[en.node] = append(byNode[en.node], en)
	}

	listening := nodes.Listening()
	known := map[string]bool{}
	add := func(tag, proto string, port int) {
		known[tag] = true
		nv := NodeView{Tag: tag, Proto: proto, Port: port, Limit: e.limitFor(tag), Listening: listening[port]}
		for _, en := range byNode[tag] {
			if len(en.conns) == 0 {
				continue
			}
			until, cooling := e.st.Cooldown(key(en.node, en.ipKey))
			allowed := e.whitelisted(en.ip)
			nv.IPs = append(nv.IPs, IPView{
				IP:          en.ip,
				Conns:       len(en.conns),
				Up:          en.up,
				Down:        en.down,
				ActiveAgo:   int64(now.Sub(en.lastActive).Seconds()),
				Cooling:     cooling && until.After(now),
				Whitelisted: allowed,
			})
			if !allowed {
				nv.Online++
			}
		}
		sort.Slice(nv.IPs, func(i, j int) bool { return nv.IPs[i].ActiveAgo < nv.IPs[j].ActiveAgo })
		snap.Nodes = append(snap.Nodes, nv)
	}

	if e.list != nil {
		for _, n := range e.list() {
			if !known[n.Tag] {
				add(n.Tag, n.Proto, n.Port)
			}
		}
	}
	for tag := range byNode {
		if !known[tag] {
			add(tag, "", 0)
		}
	}
	sort.Slice(snap.Nodes, func(i, j int) bool { return snap.Nodes[i].Port < snap.Nodes[j].Port })
	return snap
}

func (e *Engine) SetLimit(node string, limit int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.st.SetLimit(node, limit); err != nil {
		return err
	}
	// Changing the limit is an explicit admin action, so lift existing blocks
	// on that node right away instead of waiting out the cooldown.
	if err := e.st.ClearNodeCooldowns(node); err != nil {
		return err
	}
	e.event(e.now(), "limit", node, "", fmt.Sprintf("上限改为 %d（已解除该节点的冷却）", limit))
	return nil
}

func (e *Engine) AllowAdd(entry string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.st.AllowAdd(entry); err != nil {
		return err
	}
	for _, en := range e.entries {
		if ipkey.Match(entry, en.ip) {
			if err := e.st.ClearCooldown(key(en.node, en.ipKey)); err != nil {
				return err
			}
		}
	}
	e.event(e.now(), "allow", "", entry, "加入白名单（已解除该 IP 的冷却）")
	return nil
}

func (e *Engine) AllowDel(entry string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ok, err := e.st.AllowDel(entry)
	if err == nil && ok {
		e.event(e.now(), "allow", "", entry, "移出白名单")
	}
	return ok, err
}

// Kick drops every live connection of ip, on one node or on all of them, and
// puts the IP into cooldown so it stays out for the full window.
func (e *Engine) Kick(ctx context.Context, ip, node string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := e.now()
	k := ipkey.Key(ip, e.opts.IPv6Prefix)
	dropped, matched := 0, 0
	for _, en := range e.entries {
		if en.ipKey != k {
			continue
		}
		if node != "" && en.node != node {
			continue
		}
		matched++
		dropped += e.drop(ctx, en)
		if err := e.st.SetCooldown(key(en.node, en.ipKey), now.Add(e.opts.Cooldown)); err != nil {
			return dropped, err
		}
		e.event(now, "kick", en.node, en.ip, fmt.Sprintf("手动踢出，断开 %d 条连接", dropped))
	}
	if matched == 0 {
		if node == "" {
			return 0, fmt.Errorf("没有找到 %s 的在线连接，请用 --node 指定节点", ip)
		}
		if err := e.st.SetCooldown(key(node, k), now.Add(e.opts.Cooldown)); err != nil {
			return 0, err
		}
		e.event(now, "kick", node, ip, fmt.Sprintf("当前无连接，仍冷却 %s", e.opts.Cooldown))
	} else if dropped == 0 {
		e.event(now, "kick", node, ip, fmt.Sprintf("连接已自行消失，仍冷却 %s", e.opts.Cooldown))
	}
	return dropped, nil
}
