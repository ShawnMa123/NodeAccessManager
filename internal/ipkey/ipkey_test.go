package ipkey

import "testing"

func TestKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"::ffff:203.0.113.7", "203.0.113.7"},
		{"2001:db8:1234:5678:8c03:81ff:fe71:6641", "2001:db8:1234:5678::/64"},
		{"2001:db8:1234:5678:1:2:3:4", "2001:db8:1234:5678::/64"},
		{"garbage", "garbage"},
	}
	for _, c := range cases {
		if got := Key(c.in, 64); got != c.want {
			t.Errorf("Key(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMatch(t *testing.T) {
	if !Match("203.0.113.7", "203.0.113.7") {
		t.Error("exact address should match")
	}
	if Match("203.0.113.8", "203.0.113.7") {
		t.Error("different address must not match")
	}
	if !Match("198.51.100.0/24", "198.51.100.42") {
		t.Error("cidr should match address inside it")
	}
	if !Match("2001:db8:1234:5678::/64", "2001:db8:1234:5678:8c03:81ff:fe71:6641") {
		t.Error("v6 prefix should match address inside it")
	}
}
