// Command nam is both the CLI and the daemon. `nam` alone opens the interactive
// menu, `nam daemon` is what systemd runs.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ShawnMa123/NodeAccessManager/internal/cfg"
	"github.com/ShawnMa123/NodeAccessManager/internal/clash"
	"github.com/ShawnMa123/NodeAccessManager/internal/engine"
	"github.com/ShawnMa123/NodeAccessManager/internal/ipc"
	"github.com/ShawnMa123/NodeAccessManager/internal/nodes"
	"github.com/ShawnMa123/NodeAccessManager/internal/state"
	"github.com/ShawnMa123/NodeAccessManager/internal/ui"
)

var Version = "dev"

func main() {
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		ui.Color = false
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, ui.Red("错误: ")+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	c, err := cfg.Load(cfg.Path())
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return menu(c)
	}
	switch args[0] {
	case "daemon":
		return daemon(c)
	case "setup":
		return setup(c)
	case "doctor":
		return doctor(c)
	case "status":
		return status(c, len(args) > 1 && args[1] == "--json")
	case "policy":
		return policyCmd(c, args[1:])
	case "allow":
		return allowCmd(c, args[1:])
	case "kick":
		return kickCmd(c, args[1:])
	case "version", "-v", "--version":
		fmt.Println("nam", Version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("未知命令: %s", args[0])
	}
}

func usage() {
	fmt.Print(`nam — sing-box 节点 IP 访问控制

  nam                     交互菜单
  nam status [--json]     查看各节点在线 IP
  nam policy set <节点> <上限>
  nam policy list
  nam allow add|del|list <IP或CIDR>
  nam kick <IP> [--node <节点>]
  nam doctor              自检
  nam setup               为 sing-box 开启 Clash API（首次安装用）
  nam daemon              守护进程（systemd 调用）

节点参数可以写 inbound tag，也可以直接写端口号。
配置文件: ` + cfg.Path() + `
`)
}

// ---------------------------------------------------------------- daemon

func daemon(c cfg.Config) error {
	api, err := newAPI(c)
	if err != nil {
		return err
	}
	st, err := state.Load(c.StateFile)
	if err != nil {
		return fmt.Errorf("读取状态文件 %s: %w", c.StateFile, err)
	}
	list := func() []nodes.Node {
		n, err := nodes.Discover(c.SingboxConfDir)
		if err != nil {
			return nil
		}
		return n
	}
	opts := engine.Options{
		PollInterval: time.Duration(c.PollSeconds) * time.Second,
		OnlineTTL:    time.Duration(c.OnlineTTLSeconds) * time.Second,
		Cooldown:     time.Duration(c.CooldownSeconds) * time.Second,
		DefaultLimit: c.DefaultLimit,
		IPv6Prefix:   c.IPv6Prefix,
	}
	e := engine.New(opts, api, st, list)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go e.Run(ctx)

	// SIGHUP re-reads the sing-box config, so a rotated Clash API secret or
	// address is picked up without restarting the daemon.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			if fresh, err := newAPI(c); err == nil {
				e.SetAPI(fresh)
				fmt.Fprintln(os.Stderr, "nam: 已重新加载 Clash API 配置")
			} else {
				fmt.Fprintln(os.Stderr, "nam: 重新加载失败:", err)
			}
		}
	}()

	fmt.Fprintf(os.Stderr, "nam: 守护进程启动，socket=%s 轮询=%s 默认上限=%d\n",
		c.Socket, opts.PollInterval, opts.DefaultLimit)
	return ipc.Serve(ctx, c.Socket, handler(e))
}

func handler(e *engine.Engine) ipc.Handler {
	return func(ctx context.Context, req ipc.Request) ipc.Response {
		switch req.Cmd {
		case "ping":
			return ipc.Response{OK: true, Msg: "pong"}
		case "status":
			s := e.Status()
			return ipc.Response{OK: true, Status: &s}
		case "limit":
			if err := e.SetLimit(req.Node, req.Limit); err != nil {
				return ipc.Response{Err: err.Error()}
			}
			return ipc.Response{OK: true, Msg: fmt.Sprintf("%s 的上限已设为 %d", req.Node, req.Limit)}
		case "allow_add":
			if err := e.AllowAdd(req.IP); err != nil {
				return ipc.Response{Err: err.Error()}
			}
			return ipc.Response{OK: true, Msg: req.IP + " 已加入白名单"}
		case "allow_del":
			ok, err := e.AllowDel(req.IP)
			if err != nil {
				return ipc.Response{Err: err.Error()}
			}
			if !ok {
				return ipc.Response{Err: req.IP + " 不在白名单里"}
			}
			return ipc.Response{OK: true, Msg: req.IP + " 已移出白名单"}
		case "kick":
			n, err := e.Kick(ctx, req.IP, req.Node)
			if err != nil {
				return ipc.Response{Err: err.Error()}
			}
			if n == 0 {
				return ipc.Response{OK: true, Msg: fmt.Sprintf("%s 当前无活动连接，已进入冷却", req.IP)}
			}
			return ipc.Response{OK: true, Msg: fmt.Sprintf("已断开 %d 条连接，%s 进入冷却", n, req.IP)}
		default:
			return ipc.Response{Err: "未知命令: " + req.Cmd}
		}
	}
}

func newAPI(c cfg.Config) (*clash.Client, error) {
	addr, secret, err := cfg.ClashAPI(c.SingboxConfig)
	if err != nil {
		return nil, fmt.Errorf("读取 %s: %w", c.SingboxConfig, err)
	}
	if addr == "" {
		return nil, fmt.Errorf("%s 里没有 experimental.clash_api，先执行: nam setup", c.SingboxConfig)
	}
	return clash.New(cfg.URL(addr), secret), nil
}

// ---------------------------------------------------------------- commands

func status(c cfg.Config, asJSON bool) error {
	resp, err := ipc.Call(context.Background(), c.Socket, ipc.Request{Cmd: "status"})
	if err != nil {
		return err
	}
	if asJSON {
		b, err := json.MarshalIndent(resp.Status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(ui.Status(*resp.Status))
	return nil
}

func call(c cfg.Config, req ipc.Request) (ipc.Response, error) {
	return ipc.Call(context.Background(), c.Socket, req)
}

func policyCmd(c cfg.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: nam policy set <节点> <上限> | nam policy list")
	}
	switch args[0] {
	case "list":
		resp, err := call(c, ipc.Request{Cmd: "status"})
		if err != nil {
			return err
		}
		for _, n := range resp.Status.Nodes {
			fmt.Printf("%-34s %-8s %-8s 上限 %d  在线 %d\n", n.Tag, n.Proto, portString(n.Port), n.Limit, n.Online)
		}
		return nil
	case "set":
		if len(args) < 3 {
			return fmt.Errorf("用法: nam policy set <节点> <上限>")
		}
		limit, err := strconv.Atoi(args[2])
		if err != nil || limit <= 0 {
			return fmt.Errorf("上限必须是正整数: %s", args[2])
		}
		tag, err := resolveNode(c, args[1])
		if err != nil {
			return err
		}
		resp, err := call(c, ipc.Request{Cmd: "limit", Node: tag, Limit: limit})
		if err != nil {
			return err
		}
		fmt.Println(resp.Msg)
		return nil
	default:
		return fmt.Errorf("用法: nam policy set <节点> <上限> | nam policy list")
	}
}

func allowCmd(c cfg.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: nam allow add|del|list <IP或CIDR>")
	}
	switch args[0] {
	case "list":
		resp, err := call(c, ipc.Request{Cmd: "status"})
		if err != nil {
			return err
		}
		if len(resp.Status.Whitelist) == 0 {
			fmt.Println("白名单为空")
			return nil
		}
		for _, w := range resp.Status.Whitelist {
			fmt.Println(w)
		}
		return nil
	case "add", "del":
		if len(args) < 2 {
			return fmt.Errorf("用法: nam allow %s <IP或CIDR>", args[0])
		}
		if err := validateIPOrCIDR(args[1]); err != nil {
			return err
		}
		cmd := "allow_add"
		if args[0] == "del" {
			cmd = "allow_del"
		}
		resp, err := call(c, ipc.Request{Cmd: cmd, IP: args[1]})
		if err != nil {
			return err
		}
		fmt.Println(resp.Msg)
		return nil
	default:
		return fmt.Errorf("用法: nam allow add|del|list <IP或CIDR>")
	}
}

func kickCmd(c cfg.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: nam kick <IP> [--node <节点>]")
	}
	ip := args[0]
	node := ""
	for i := 1; i < len(args); i++ {
		if (args[i] == "--node" || args[i] == "-n") && i+1 < len(args) {
			tag, err := resolveNode(c, args[i+1])
			if err != nil {
				return err
			}
			node = tag
			i++
		}
	}
	if err := validateIPOrCIDR(ip); err != nil {
		return err
	}
	resp, err := call(c, ipc.Request{Cmd: "kick", IP: ip, Node: node})
	if err != nil {
		return err
	}
	fmt.Println(resp.Msg)
	return nil
}

// resolveNode accepts an inbound tag, a full tag, or a bare port number.
func resolveNode(c cfg.Config, arg string) (string, error) {
	all, err := nodes.Discover(c.SingboxConfDir)
	if err != nil {
		return "", fmt.Errorf("读取 %s: %w", c.SingboxConfDir, err)
	}
	if port, err := strconv.Atoi(arg); err == nil {
		for _, n := range all {
			if n.Port == port {
				return n.Tag, nil
			}
		}
		return "", fmt.Errorf("没有端口为 %d 的节点", port)
	}
	for _, n := range all {
		if n.Tag == arg || strings.TrimSuffix(n.Tag, ".json") == arg {
			return n.Tag, nil
		}
	}
	return "", fmt.Errorf("没有名为 %s 的节点，用 nam status 查看", arg)
}

// validateIPOrCIDR rejects typos before they reach the daemon and the state
// file.
func validateIPOrCIDR(s string) error {
	if _, err := netip.ParsePrefix(s); err == nil {
		return nil
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return nil
	}
	return fmt.Errorf("不是合法的 IP 或 CIDR: %s", s)
}

func portString(p int) string {
	if p == 0 {
		return "?"
	}
	return ":" + strconv.Itoa(p)
}

// ---------------------------------------------------------------- setup

// setup makes sing-box expose the Clash API that NAM needs. It only fills in
// missing fields, keeps a backup, and never restarts sing-box itself.
func setup(c cfg.Config) error {
	addr, secret, err := cfg.ClashAPI(c.SingboxConfig)
	if err != nil {
		return fmt.Errorf("读取 %s: %w（sing-box 装好了吗？）", c.SingboxConfig, err)
	}
	if addr != "" {
		if !cfg.IsLoopback(addr) {
			fmt.Println(ui.Yellow("注意: Clash API 监听在 ") + addr + ui.Yellow("，不是仅本机。没有 secret 就等于把控制口暴露到公网。"))
		}
		if secret == "" {
			fmt.Println(ui.Yellow("注意: Clash API 没有设置 secret，建议补一个。"))
		} else if len(secret) < 16 {
			fmt.Println(ui.Yellow("注意: Clash API 的 secret 太短（" + secret + "），建议换成 32 位随机字符串。"))
		}
		fmt.Printf("Clash API 已经配置: %s\n", addr)
		if err := probe(c); err != nil {
			fmt.Printf("%s %v\n", ui.Red("但是连不上:"), err)
			fmt.Println("提示: 改过 config.json 之后需要重启 sing-box 才生效（sb → 5) 运行管理 → 重启）")
			return nil
		}
		fmt.Println(ui.Green("API 可用。"))
		return nil
	}

	newSecret := secret
	if newSecret == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return err
		}
		newSecret = hex.EncodeToString(buf)
	}
	changed, err := cfg.EnsureClashAPI(c.SingboxConfig, "127.0.0.1:9090", newSecret)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("没有需要修改的内容。")
		return nil
	}
	fmt.Println(ui.Green("已写入 experimental.clash_api:"))
	fmt.Println("  external_controller: 127.0.0.1:9090")
	fmt.Println("  secret:              " + newSecret)
	fmt.Println("原文件已备份为 " + c.SingboxConfig + ".nam.bak")
	fmt.Println()
	fmt.Println(ui.Yellow("现在重启一次 sing-box 让配置生效:"))
	fmt.Println("  sb  →  5) 运行管理  →  重启      （或 systemctl restart sing-box）")
	fmt.Println("然后执行 nam doctor 验证。")
	return nil
}

func probe(c cfg.Config) error {
	api, err := newAPI(c)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conns, err := api.Connections(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Clash API 正常，当前连接数 %d\n", len(conns))
	return nil
}

// ---------------------------------------------------------------- doctor

func doctor(c cfg.Config) error {
	type check struct {
		level  string // ok | warn | fail
		name   string
		detail string
	}
	var checks []check
	add := func(level, name, detail string) {
		checks = append(checks, check{level, name, detail})
	}
	failed := false
	finish := func() {
		fmt.Println()
		for _, ch := range checks {
			var tag string
			switch ch.level {
			case "ok":
				tag = ui.Green("[通过]")
			case "warn":
				tag = ui.Yellow("[警告]")
			default:
				tag = ui.Red("[失败]")
				failed = true
			}
			fmt.Printf("%s %s: %s\n", tag, ch.name, ch.detail)
		}
		if failed {
			os.Exit(1)
		}
	}

	addr, secret, err := cfg.ClashAPI(c.SingboxConfig)
	switch {
	case err != nil:
		add("fail", "sing-box 配置", fmt.Sprintf("读不到 %s: %v", c.SingboxConfig, err))
	case addr == "":
		add("fail", "Clash API", "没有配置，执行 nam setup")
	case !cfg.IsLoopback(addr):
		add("warn", "Clash API 地址", addr+" 不是仅本机监听")
		if secret == "" {
			add("fail", "Clash API secret", "对外监听且没有 secret")
		} else {
			add("ok", "Clash API secret", "已设置")
		}
	default:
		add("ok", "Clash API", addr+"（仅本机）")
		if secret == "" {
			add("warn", "Clash API secret", "未设置")
		} else {
			add("ok", "Clash API secret", "已设置")
		}
	}

	if addr != "" {
		api := clash.New(cfg.URL(addr), secret)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		conns, err := api.Connections(ctx)
		if err != nil {
			add("fail", "Clash API 连通性", fmt.Sprintf("%v（改过配置的话先重启 sing-box）", err))
		} else {
			add("ok", "Clash API 连通性", fmt.Sprintf("%d 条在线连接", len(conns)))
			if err := api.SupportsDelete(ctx); err != nil {
				add("fail", "断连接口", fmt.Sprintf("DELETE /connections 不可用: %v", err))
			} else {
				add("ok", "断连接口", "DELETE /connections 可用")
			}
		}
		cancel()
	}

	list, err := nodes.Discover(c.SingboxConfDir)
	if err != nil {
		add("fail", "节点目录", fmt.Sprintf("读不到 %s: %v", c.SingboxConfDir, err))
	} else if len(list) == 0 {
		add("fail", "节点目录", "没有发现任何节点")
	} else {
		listening := nodes.Listening()
		var desc []string
		dead := 0
		for _, n := range list {
			s := fmt.Sprintf("%d:%s", n.Port, n.Tag)
			if !listening[n.Port] {
				s += "(未监听)"
				dead++
			}
			desc = append(desc, s)
		}
		detail := fmt.Sprintf("%d 个节点 %s", len(list), strings.Join(desc, "  "))
		if dead > 0 {
			add("warn", "节点", detail+"  sing-box 是不是停了？")
		} else {
			add("ok", "节点", detail)
		}
	}

	dir := filepath.Dir(c.StateFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		add("fail", "状态目录", err.Error())
	} else if f, err := os.CreateTemp(dir, ".nam-doctor-*"); err != nil {
		add("fail", "状态目录", err.Error())
	} else {
		f.Close()
		os.Remove(f.Name())
		add("ok", "状态目录", dir+" 可写")
	}

	if resp, err := call(c, ipc.Request{Cmd: "ping"}); err != nil {
		add("warn", "守护进程", "没有响应（systemctl start nam）: "+err.Error())
	} else {
		add("ok", "守护进程", resp.Msg)
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() == nil && ipnet.IP.IsGlobalUnicast() {
				add("ok", "IPv6", ipnet.String()+"（同一 /"+strconv.Itoa(c.IPv6Prefix)+" 按一个 IP 计）")
				break
			}
		}
	}

	finish()
	return nil
}

// ---------------------------------------------------------------- menu

func menu(c cfg.Config) error {
	for {
		fmt.Print("\033[H\033[2J") // clear screen; harmless if the terminal ignores it
		resp, err := call(c, ipc.Request{Cmd: "status"})
		if err != nil {
			fmt.Println(ui.Red("守护进程未运行") + "  " + err.Error())
		} else {
			fmt.Print(ui.Status(*resp.Status))
		}
		fmt.Println()
		fmt.Println(ui.Bold("1") + ") 刷新   " + ui.Bold("2") + ") 修改节点上限   " +
			ui.Bold("3") + ") 白名单   " + ui.Bold("4") + ") 手动踢人   " +
			ui.Bold("5") + ") 自检   " + ui.Bold("0") + ") 退出")
		switch ui.Prompt("请选择: ") {
		case "0", "q", "exit":
			return nil
		case "2":
			menuLimit(c)
		case "3":
			menuAllow(c)
		case "4":
			menuKick(c)
		case "5":
			_ = doctor(c)
			ui.Prompt("回车继续")
		}
	}
}

func menuLimit(c cfg.Config) error {
	resp, err := call(c, ipc.Request{Cmd: "status"})
	if err != nil {
		return err
	}
	for i, n := range resp.Status.Nodes {
		fmt.Printf("  %d) %s (%s) 上限 %d 在线 %d\n", i+1, n.Tag, portString(n.Port), n.Limit, n.Online)
	}
	choice := ui.Prompt("选择节点编号: ")
	i, err := strconv.Atoi(choice)
	if err != nil || i < 1 || i > len(resp.Status.Nodes) {
		return nil
	}
	limit, err := strconv.Atoi(ui.Prompt("新的上限: "))
	if err != nil || limit <= 0 {
		fmt.Println(ui.Red("上限必须是正整数"))
		return nil
	}
	r, err := call(c, ipc.Request{Cmd: "limit", Node: resp.Status.Nodes[i-1].Tag, Limit: limit})
	if err != nil {
		fmt.Println(ui.Red(err.Error()))
		return nil
	}
	fmt.Println(r.Msg)
	return nil
}

func menuAllow(c cfg.Config) error {
	resp, err := call(c, ipc.Request{Cmd: "status"})
	if err != nil {
		return err
	}
	fmt.Println("当前白名单:", strings.Join(resp.Status.Whitelist, ", "))
	entry := ui.Prompt("输入 IP 或 CIDR（留空返回）: ")
	if entry == "" {
		return nil
	}
	action := ui.Prompt("1) 加入  2) 移除  [1]: ")
	cmd := "allow_add"
	if action == "2" {
		cmd = "allow_del"
	}
	r, err := call(c, ipc.Request{Cmd: cmd, IP: entry})
	if err != nil {
		fmt.Println(ui.Red(err.Error()))
		return nil
	}
	fmt.Println(r.Msg)
	return nil
}

func menuKick(c cfg.Config) error {
	ip := ui.Prompt("要踢掉的 IP（留空返回）: ")
	if ip == "" {
		return nil
	}
	node := ui.Prompt("只踢某个节点？输入端口或直接回车=全部: ")
	tag := ""
	if node != "" {
		t, err := resolveNode(c, node)
		if err != nil {
			fmt.Println(ui.Red(err.Error()))
			return nil
		}
		tag = t
	}
	r, err := call(c, ipc.Request{Cmd: "kick", IP: ip, Node: tag})
	if err != nil {
		fmt.Println(ui.Red(err.Error()))
		return nil
	}
	fmt.Println(r.Msg)
	return nil
}
