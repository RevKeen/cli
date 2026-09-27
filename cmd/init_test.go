package cmd

import (
	"bytes"
	"errors"
	"testing"
)

// REV-8396: `revkeen init` is a tombstone for one release. Every invocation,
// with or without --cms, must fail with exactly the replacement message and
// print nothing else (main.go prints the error once).
func TestInitIsReplacedBySanityRemovalMessage(t *testing.T) {
	cases := [][]string{
		{"init"},
		{"init", "--cms", "sanity"},
		{"init", "--cms", "contentful"},
		{"init", "--cms", "sanity", "--framework", "nextjs", "--deploy", "vercel", "--path", ".", "--dry-run", "--yes"},
	}
	for _, args := range cases {
		t.Setenv("HOME", t.TempDir())
		cmd := NewRootCmd()
		var stdout, stderr bytes.Buffer
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)

		err := cmd.Execute()
		if !errors.Is(err, errSanityIntegrationReplaced) {
			t.Fatalf("%v: expected the replacement error, got %v", args, err)
		}
		if err.Error() != "The Sanity integration was replaced by RevKeen Storefront" {
			t.Fatalf("%v: message drifted: %q", args, err.Error())
		}
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%v: expected no extra output, got stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}
}
