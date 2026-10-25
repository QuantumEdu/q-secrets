package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/QuantumEdu/q-secrets/internal"
)

func validateOutput(format string, values bool) error {
	switch format {
	case "json", "table":
		return nil
	case "env":
		if values {
			return nil
		}
		return fmt.Errorf("list never reveals values; use get or export for env output")
	default:
		return fmt.Errorf("invalid output format: use json, env or table (export file paths use --file)")
	}
}

// renderSecrets writes once after validation, so collisions never produce a
// partial env export. Env uses POSIX shell single quotes, including newlines.
func renderSecrets(w io.Writer, records []exportSecret, format string, values bool) error {
	if err := validateOutput(format, values); err != nil {
		return err
	}
	var out strings.Builder
	switch format {
	case "json":
		rows := make([]map[string]string, 0, len(records))
		for _, row := range records {
			item := map[string]string{"project": row.Project}
			if row.Key != "" {
				item["key"] = row.Key
			}
			if values {
				item["value"] = row.Value
			}
			rows = append(rows, item)
		}
		enc := json.NewEncoder(&out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return err
		}
	case "env":
		seen := make(map[string]bool)
		for _, row := range records {
			if err := internal.ValidateEnvName(row.Key); err != nil {
				return err
			}
			if strings.ContainsRune(row.Value, 0) {
				return fmt.Errorf("value for %s contains NUL", row.Key)
			}
			if seen[row.Key] {
				return fmt.Errorf("duplicate key %q across projects; export with --project", row.Key)
			}
			seen[row.Key] = true
			fmt.Fprintf(&out, "%s='%s'\n", row.Key, strings.ReplaceAll(row.Value, "'", "'\"'\"'"))
		}
	case "table":
		table := tabwriter.NewWriter(&out, 0, 4, 2, ' ', 0)
		if values {
			fmt.Fprintln(table, "PROJECT\tKEY\tVALUE")
		} else {
			fmt.Fprintln(table, "PROJECT\tKEY")
		}
		for _, row := range records {
			fmt.Fprintf(table, "%s\t%s", strconv.Quote(row.Project), strconv.Quote(row.Key))
			if values {
				fmt.Fprintf(table, "\t%s", strconv.Quote(row.Value))
			}
			fmt.Fprintln(table)
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, out.String())
	return err
}
