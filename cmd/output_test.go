package cmd

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"testing"
)

func TestOutputFormats(t *testing.T) {
	rows := []exportSecret{{Project: "p", Key: "TOKEN", Value: "quote'\n$PATH `id` \\"}}
	var out bytes.Buffer
	if err := renderSecrets(&out, rows, "json", true); err != nil {
		t.Fatal(err)
	}
	var roundtrip []exportSecret
	if err := json.Unmarshal(out.Bytes(), &roundtrip); err != nil || roundtrip[0] != rows[0] {
		t.Fatal("JSON round trip failed")
	}
	out.Reset()
	if err := renderSecrets(&out, rows, "table", false); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte("quote")) {
		t.Fatal("list leaked secret")
	}
	out.Reset()
	if err := renderSecrets(&out, rows, "env", true); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("sh"); err == nil {
		child := exec.Command("sh", "-c", out.String()+"\nprintf '%s' \"$TOKEN\"")
		value, err := child.Output()
		if err != nil || string(value) != rows[0].Value {
			t.Fatal("unsafe env quoting")
		}
	}
}

func TestEnvCollisionWritesNothing(t *testing.T) {
	var out bytes.Buffer
	rows := []exportSecret{{Project: "a", Key: "TOKEN", Value: "a"}, {Project: "b", Key: "TOKEN", Value: "b"}}
	if err := renderSecrets(&out, rows, "env", true); err == nil || out.Len() != 0 {
		t.Fatal("collision produced partial output")
	}
	if err := renderSecrets(&out, nil, "json", false); err != nil || out.String() != "[]\n" {
		t.Fatal("empty JSON must be an array")
	}
}
