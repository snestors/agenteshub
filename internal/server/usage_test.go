package server

import "testing"

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"127.0.0.1:54321", true},
		{"127.0.0.1", true},
		{"[::1]:8080", true},
		{"::1", true},
		{"127.5.5.5:1", true},
		{"192.168.1.62:9000", false},
		{"10.0.0.1", false},
		{"0.0.0.0", false},
		{"", false},
		{"garbage", false},
	}
	for _, tc := range cases {
		if got := isLoopbackAddr(tc.in); got != tc.want {
			t.Errorf("isLoopbackAddr(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
