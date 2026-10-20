// Package clash is a minimal client for the sing-box Clash API. Only the two
// endpoints NAM needs: the live connection list and closing one connection.
package clash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	base   string
	secret string
	hc     *http.Client
}

func New(base, secret string) *Client {
	return &Client{
		base:   base,
		secret: secret,
		hc:     &http.Client{Timeout: 5 * time.Second},
	}
}

type Conn struct {
	ID       string    `json:"id"`
	Upload   int64     `json:"upload"`
	Download int64     `json:"download"`
	Start    time.Time `json:"start"`
	Metadata struct {
		Network       string `json:"network"`
		Type          string `json:"type"`
		SourceIP      string `json:"sourceIP"`
		SourcePort    string `json:"sourcePort"`
		DestinationIP string `json:"destinationIP"`
		Host          string `json:"host"`
	} `json:"metadata"`
}

// NodeTag returns the sing-box inbound tag from metadata.type, which the API
// writes as "<protocol>/<inbound tag>".
func (c Conn) NodeTag() string {
	if i := strings.IndexByte(c.Metadata.Type, '/'); i >= 0 {
		return c.Metadata.Type[i+1:]
	}
	return c.Metadata.Type
}

func (c Conn) Proto() string {
	if i := strings.IndexByte(c.Metadata.Type, '/'); i >= 0 {
		return c.Metadata.Type[:i]
	}
	return ""
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Connections returns every connection sing-box is currently proxying. Failed
// handshakes and port scanners never appear here, which is what makes it a
// sound source for "who is really online".
func (c *Client) Connections(ctx context.Context) ([]Conn, error) {
	var out struct {
		Connections []Conn `json:"connections"`
	}
	if err := c.get(ctx, "/connections", &out); err != nil {
		return nil, err
	}
	return out.Connections, nil
}

// CloseConn drops one live connection.
func (c *Client) CloseConn(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.base+"/connections/"+id, nil)
	if err != nil {
		return err
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("DELETE /connections/%s: HTTP %d", id, resp.StatusCode)
	}
	return nil
}

// SupportsDelete probes the DELETE endpoint without touching a real
// connection: an unknown id answers 204 or 404, while a missing endpoint
// answers 405/501. Used by `nam doctor`.
func (c *Client) SupportsDelete(ctx context.Context) error {
	err := c.CloseConn(ctx, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "HTTP 404") {
		return nil
	}
	return err
}
