// Package ui renders the status table and the interactive menu. Plain ANSI on
// purpose: it has to work over ssh and inside tmux on a 1-core box.
package ui

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ShawnMa123/NodeAccessManager/internal/engine"
)

var Color = true

func wrap(code, s string) string {
	if !Color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func Green(s string) string  { return wrap("32", s) }
func Red(s string) string    { return wrap("31", s) }
func Yellow(s string) string { return wrap("33", s) }
func Dim(s string) string    { return wrap("90", s) }
func Bold(s string) string   { return wrap("1", s) }

func HumanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit && exp < 4; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTP"[exp])
}

func Ago(seconds int64) string {
	switch {
	case seconds < 2:
		return "刚刚"
	case seconds < 60:
		return fmt.Sprintf("%ds 前", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	default:
		return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
	}
}

func Duration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Second {
		return d.String()
	}
	return d.String()
}

// Status renders the whole daemon state as text.
func Status(s engine.Snapshot) string {
	var b strings.Builder

	state := Green("运行中")
	if !s.APIOK {
		state = Red("Clash API 不可用")
	}
	fmt.Fprintf(&b, "%s  运行 %s  默认上限 %d  %s\n",
		Bold("NAM"), Duration(s.Now.Sub(s.Started)), s.DefaultLimit, state)
	if s.APIOK {
		fmt.Fprintf(&b, "%s\n", Dim("最后轮询 "+Ago(int64(s.Now.Sub(s.LastPoll).Seconds()))))
	}
	if s.APIErr != "" {
		fmt.Fprintf(&b, "%s %s\n", Red("API 错误:"), s.APIErr)
	}
	if len(s.Whitelist) > 0 {
		fmt.Fprintf(&b, "白名单: %s\n", strings.Join(s.Whitelist, ", "))
	} else {
		fmt.Fprintf(&b, "白名单: %s\n", Dim("(空)"))
	}

	for _, n := range s.Nodes {
		port := "端口 ?"
		if n.Port > 0 {
			port = fmt.Sprintf(":%d", n.Port)
			if !n.Listening {
				port += Red(" 未监听")
			}
		}
		online := fmt.Sprintf("在线 %d/%d", n.Online, n.Limit)
		if n.Online > n.Limit {
			online = Red(online)
		} else if n.Online == n.Limit {
			online = Yellow(online)
		}
		fmt.Fprintf(&b, "\n%s  %s %s  %s\n", Bold(n.Tag), Dim(n.Proto), Dim(port), online)

		if len(n.IPs) == 0 {
			fmt.Fprintf(&b, "  %s\n", Dim("无 IP 在线"))
			continue
		}
		tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  IP\t连接\t上行\t下行\t活跃\t状态")
		for _, ip := range n.IPs {
			status := "正常"
			if ip.Whitelisted {
				status = Dim("白名单")
			} else if ip.Cooling {
				status = Yellow("冷却中")
			}
			fmt.Fprintf(tw, "  %s\t%d\t%s\t%s\t%s\t%s\n",
				ip.IP, ip.Conns, HumanBytes(ip.Up), HumanBytes(ip.Down), Ago(ip.ActiveAgo), status)
		}
		tw.Flush()
	}

	if len(s.Events) > 0 {
		fmt.Fprintf(&b, "\n%s\n", Bold("最近事件"))
		limit := len(s.Events)
		if limit > 10 {
			limit = 10
		}
		for _, e := range s.Events[:limit] {
			label := Dim("事件")
			switch e.Kind {
			case "kick":
				label = Red("踢出")
			case "limit":
				label = Yellow("限额")
			case "allow":
				label = Green("白名单")
			case "api":
				label = Red("API")
			}
			line := fmt.Sprintf("  %s %s  %s", Dim(e.At.Format("15:04:05")), label, e.Note)
			if e.Node != "" {
				line += Dim(" [" + e.Node + "]")
			}
			if e.IP != "" {
				line += " " + e.IP
			}
			fmt.Fprintln(&b, line)
		}
	}
	return b.String()
}

// Prompt asks one line of input on stderr.
func Prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	var line string
	fmt.Fscanln(os.Stdin, &line)
	return strings.TrimSpace(line)
}
