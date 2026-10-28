package main

import "testing"

func TestValidateLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "[::1]:8080", "localhost:9000"} {
		if err := validateLoopback(address); err != nil {
			t.Fatalf("validateLoopback(%q) = %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "example.com:8080", "127.0.0.1"} {
		if err := validateLoopback(address); err == nil {
			t.Fatalf("validateLoopback(%q) accepted unsafe address", address)
		}
	}
}
