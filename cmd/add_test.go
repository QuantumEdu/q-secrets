package cmd

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func inputCommand(stdin, argv bool, reader io.Reader) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("stdin", stdin, "")
	cmd.Flags().Bool("allow-argv", argv, "")
	cmd.SetIn(reader)
	return cmd
}

func TestStdinExactValue(t *testing.T) {
	for _, value := range []string{"", "line1\nline2\n", "a=b 'quote'"} {
		cmd := inputCommand(true, false, strings.NewReader(value))
		got, err := readSecretInput(cmd, []string{"API_KEY"})
		if err != nil || got["API_KEY"] != value {
			t.Fatalf("stdin changed value: %v", err)
		}
	}
}

func TestArgumentSafety(t *testing.T) {
	for _, args := range [][]string{{"TOKEN=sensitive"}, {"BAD-NAME"}, {"Q_SECRET_KEY"}, {"KEY", "OTHER"}} {
		cmd := inputCommand(true, false, strings.NewReader("secret"))
		_, err := readSecretInput(cmd, args)
		if err == nil || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("unsafe args handling: %v", err)
		}
	}
	cmd := inputCommand(false, true, nil)
	got, err := readSecretInput(cmd, []string{"TOKEN=a=b", "EMPTY="})
	if err != nil || got["TOKEN"] != "a=b" {
		t.Fatal(err)
	}
	if _, err := readSecretInput(cmd, []string{"TOKEN=a", "TOKEN=b"}); err == nil {
		t.Fatal("accepted duplicate")
	}
	if _, err := readSecretInput(inputCommand(false, false, strings.NewReader("x")), []string{"TOKEN"}); err == nil {
		t.Fatal("nonterminal prompt accepted")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("test input error") }
func TestStdinReadError(t *testing.T) {
	if _, err := readSecretInput(inputCommand(true, false, failingReader{}), []string{"TOKEN"}); err == nil {
		t.Fatal("read error swallowed")
	}
}
