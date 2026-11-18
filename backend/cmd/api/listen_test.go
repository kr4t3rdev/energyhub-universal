package main

import "testing"

// The development issuer speaks plain HTTP, so the API must only listen on a
// loopback address while it is in use. The check is what keeps the
// development credentials on the machine that issued them.

func TestIsLoopbackListenAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8080", want: true},
		{addr: "127.0.0.2:8080", want: true},
		{addr: "[::1]:8080", want: true},
		{addr: "[0:0:0:0:0:0:0:1]:8080", want: true},
		{addr: "localhost:8080", want: false},
		{addr: ":8080", want: false},
		{addr: "0.0.0.0:8080", want: false},
		{addr: "192.168.1.10:8080", want: false},
		{addr: "127.0.0.1", want: false},
		{addr: "not-an-address", want: false},
		{addr: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.addr, func(t *testing.T) {
			if got := isLoopbackListenAddr(test.addr); got != test.want {
				t.Fatalf("isLoopbackListenAddr(%q) = %v, want %v", test.addr, got, test.want)
			}
		})
	}
}
