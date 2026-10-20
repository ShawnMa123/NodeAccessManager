// Package ipc is the unix-socket link between the `nam` CLI and the `nam
// daemon`. One JSON request per connection keeps it trivial to debug with
// socat and impossible to leave half-written.
package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/ShawnMa123/NodeAccessManager/internal/engine"
)

type Request struct {
	Cmd   string `json:"cmd"` // status | limit | allow_add | allow_del | kick | ping
	Node  string `json:"node,omitempty"`
	IP    string `json:"ip,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type Response struct {
	OK     bool             `json:"ok"`
	Err    string           `json:"err,omitempty"`
	Msg    string           `json:"msg,omitempty"`
	Status *engine.Snapshot `json:"status,omitempty"`
}

type Handler func(ctx context.Context, req Request) Response

// Serve listens on path until ctx is cancelled. A stale socket file left by a
// crashed daemon is removed first.
func Serve(ctx context.Context, path string, h Handler) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func(c net.Conn) {
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(30 * time.Second))
			var req Request
			if err := json.NewDecoder(c).Decode(&req); err != nil {
				_ = json.NewEncoder(c).Encode(Response{Err: "无法解析请求: " + err.Error()})
				return
			}
			_ = json.NewEncoder(c).Encode(h(ctx, req))
		}(conn)
	}
}

// Call sends one request to the daemon.
func Call(ctx context.Context, path string, req Request) (Response, error) {
	var resp Response
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resp, fmt.Errorf("守护进程没有运行（找不到 %s），先执行: systemctl start nam", path)
		}
		return resp, fmt.Errorf("连不上守护进程 %s: %w", path, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return resp, err
	}
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return resp, err
	}
	if !resp.OK && resp.Err != "" {
		return resp, errors.New(resp.Err)
	}
	return resp, nil
}
