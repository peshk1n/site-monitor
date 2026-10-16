package target

import (
	"context"
	"net/netip"
	"testing"
)

type resolver []netip.Addr

func (r resolver) LookupNetIP(context.Context, string, string) ([]netip.Addr, error) { return r, nil }
func TestNetworkPolicy(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "http://localhost", "http://127.0.0.1", "https://[::1]", "http://[::ffff:127.0.0.1]", "http://169.254.169.254", "http://10.0.0.1", "http://100.64.0.1", "https://user:pass@example.com", "http://example.com:0", "http://[64:ff9b::a00:1]"} {
		if ValidateURL(raw) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if ValidateURL("https://example.com/health") != nil {
		t.Fatal("public endpoint rejected")
	}
	for _, ip := range []string{"127.0.0.1", "::ffff:10.0.0.1", "::1", "169.254.1.1", "192.168.1.1"} {
		dial := DialPublic(resolver{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr(ip)})
		if _, err := dial(context.Background(), "tcp", "public.example:80"); err == nil {
			t.Fatal("DNS response containing private IP accepted")
		}
	}
}
