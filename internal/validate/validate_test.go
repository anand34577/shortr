package validate

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	private := []string{"127.0.0.1", "10.1.2.3", "192.168.0.1", "172.16.5.5", "169.254.169.254", "0.0.0.0",
		"100.64.0.1", "::1", "fc00::1", "fe80::1", "::ffff:127.0.0.1", "::ffff:10.0.0.1", "::", "224.0.0.1"}
	for _, s := range private {
		if !IsPrivateIP(net.ParseIP(s)) {
			t.Errorf("%s should be private", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if IsPrivateIP(net.ParseIP(s)) {
			t.Errorf("%s should be public", s)
		}
	}
}
