// Package ipkey maps a source address to the identity we count against a slot.
//
// IPv4: the address itself. IPv6: the /64 prefix, because a single client
// rotates privacy addresses inside its own prefix and would otherwise look
// like many different users.
package ipkey

import (
	"net/netip"
)

// Key returns the counting identity of addr.
func Key(addr string, v6bits int) string {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return addr
	}
	a = a.Unmap()
	if a.Is4() {
		return a.String()
	}
	p, err := a.Prefix(v6bits)
	if err != nil {
		return a.String()
	}
	return p.String()
}

// Match reports whether addr is covered by one whitelist entry, which may be a
// single address or a CIDR prefix.
func Match(entry, addr string) bool {
	a, err := netip.ParseAddr(addr)
	if err != nil {
		return entry == addr
	}
	a = a.Unmap()
	if p, err := netip.ParsePrefix(entry); err == nil {
		return p.Contains(a)
	}
	e, err := netip.ParseAddr(entry)
	if err != nil {
		return false
	}
	return e.Unmap() == a
}
