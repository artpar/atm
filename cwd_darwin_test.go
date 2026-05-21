//go:build darwin

package main

import "testing"

func TestParseLsofCWD(t *testing.T) {
	output := "p123\nfcwd\nn/Users/artpar/workspace/code/atm\n"
	if got := parseLsofCWD(output); got != "/Users/artpar/workspace/code/atm" {
		t.Fatalf("parseLsofCWD = %q", got)
	}
}
