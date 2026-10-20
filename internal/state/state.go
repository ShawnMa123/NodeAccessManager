// Package state is NAM's persisted policy: per-node limits, the whitelist and
// the kick cooldowns. Only the daemon writes it.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type State struct {
	mu   sync.Mutex
	path string

	Limits    map[string]int   `json:"limits"`    // node tag -> max concurrent IPs
	Whitelist []string         `json:"whitelist"` // addresses or CIDRs, never counted
	Cooldowns map[string]int64 `json:"cooldowns"` // "<node>\x00<ipkey>" -> unix expiry
}

func Load(path string) (*State, error) {
	s := &State{
		path:      path,
		Limits:    map[string]int{},
		Cooldowns: map[string]int64{},
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, s); err != nil {
		return nil, err
	}
	if s.Limits == nil {
		s.Limits = map[string]int{}
	}
	if s.Cooldowns == nil {
		s.Cooldowns = map[string]int64{}
	}
	return s, nil
}

func (s *State) save() error {
	s.mu.Lock()
	b, err := json.MarshalIndent(s, "", "  ")
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".nam-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

func (s *State) Limit(node string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Limits[node]
	return v, ok
}

func (s *State) SetLimit(node string, limit int) error {
	s.mu.Lock()
	if limit <= 0 {
		delete(s.Limits, node)
	} else {
		s.Limits[node] = limit
	}
	s.mu.Unlock()
	return s.save()
}

func (s *State) AllowAdd(entry string) error {
	s.mu.Lock()
	for _, e := range s.Whitelist {
		if e == entry {
			s.mu.Unlock()
			return nil
		}
	}
	s.Whitelist = append(s.Whitelist, entry)
	s.mu.Unlock()
	return s.save()
}

func (s *State) AllowDel(entry string) (bool, error) {
	s.mu.Lock()
	found := false
	keep := s.Whitelist[:0]
	for _, e := range s.Whitelist {
		if e == entry {
			found = true
			continue
		}
		keep = append(keep, e)
	}
	s.Whitelist = keep
	s.mu.Unlock()
	if !found {
		return false, nil
	}
	return true, s.save()
}

func (s *State) WhitelistCopy() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]string(nil), s.Whitelist...)
	sort.Strings(out)
	return out
}

func (s *State) Cooldown(key string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.Cooldowns[key]
	if !ok {
		return time.Time{}, false
	}
	return time.Unix(v, 0), true
}

func (s *State) SetCooldown(key string, until time.Time) error {
	s.mu.Lock()
	s.Cooldowns[key] = until.Unix()
	s.mu.Unlock()
	return s.save()
}

// ClearCooldown lets one IP back in immediately, used when an admin lifts a
// block by hand.
func (s *State) ClearCooldown(key string) error {
	s.mu.Lock()
	_, ok := s.Cooldowns[key]
	delete(s.Cooldowns, key)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return s.save()
}

// ClearNodeCooldowns forgets every block on one node. Raising a limit or
// whitelisting someone must take effect at once instead of after the cooldown.
func (s *State) ClearNodeCooldowns(node string) error {
	prefix := node + "\x00"
	s.mu.Lock()
	n := 0
	for k := range s.Cooldowns {
		if strings.HasPrefix(k, prefix) {
			delete(s.Cooldowns, k)
			n++
		}
	}
	s.mu.Unlock()
	if n == 0 {
		return nil
	}
	return s.save()
}

// PruneCooldowns drops expired entries and returns the keys that were removed.
func (s *State) PruneCooldowns(now time.Time) []string {
	s.mu.Lock()
	var gone []string
	for k, v := range s.Cooldowns {
		if v <= now.Unix() {
			gone = append(gone, k)
		}
	}
	for _, k := range gone {
		delete(s.Cooldowns, k)
	}
	s.mu.Unlock()
	if len(gone) > 0 {
		_ = s.save()
	}
	return gone
}
