package main

import "testing"

func TestWildcardAddressIsRejected(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:19081", "[::]:19081", ":19081"} {
		if !isWildcardAddr(addr) {
			t.Fatalf("expected wildcard address %q to be rejected", addr)
		}
	}
	if isWildcardAddr("127.0.0.1:19081") {
		t.Fatal("loopback address must be accepted")
	}
}
