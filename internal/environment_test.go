package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestChildEnv(t *testing.T) {
	parent := []string{"PATH=/bin", "PARENT_ONLY=hidden", "Q_SECRET_KEY=master", "q_secret_key=also-remove", "TOKEN=old", "TOKEN=duplicate"}
	for _, clean := range []bool{false, true} {
		got, err := ChildEnv(parent, map[string]string{"TOKEN": "new"}, clean)
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(got, "\n")
		if strings.Contains(strings.ToUpper(joined), "Q_SECRET_KEY=") || strings.Contains(joined, "TOKEN=old") || strings.Count(joined, "TOKEN=") != 1 {
			t.Fatal("master key or duplicate leaked")
		}
		if !strings.Contains(joined, "PATH=/bin") || !strings.Contains(joined, "TOKEN=new") {
			t.Fatal("missing env")
		}
		if strings.Contains(joined, "PARENT_ONLY=") == clean {
			t.Fatalf("inheritance mismatch clean=%v", clean)
		}
	}
}

func TestEnvironmentValidation(t *testing.T) {
	for _, name := range []string{"", "1BAD", "BAD-NAME", "A=B", "A\x00B", "é", "Q_SECRET_KEY", "q_secret_key"} {
		if err := ValidateEnvName(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"A", "_", "_a_012", "PATH"} {
		if err := ValidateEnvName(name); err != nil {
			t.Error(err)
		}
	}
	for _, secrets := range []map[string]string{{"Q_SECRET_KEY": "bad"}, {"BAD-NAME": "bad"}, {"GOOD": "bad\x00value"}} {
		if _, err := ChildEnv(nil, secrets, false); err == nil {
			t.Fatal("invalid secret accepted")
		}
	}
}

func TestChildEnvDeterministic(t *testing.T) {
	a, _ := ChildEnv([]string{"A=first", "A=last"}, map[string]string{"Z": "z", "B": "b"}, false)
	b, _ := ChildEnv([]string{"A=last"}, map[string]string{"B": "b", "Z": "z"}, false)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("non-deterministic environment")
	}
}

func TestBuildEnvRejectsLegacyInvalidNames(t *testing.T) {
	t.Setenv("Q_SECRET_KEY", testMasterKey)
	db, err := OpenDB(t.TempDir() + "/vault.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateProject("p"); err != nil {
		t.Fatal(err)
	}
	// Bypass the write guard to model an existing vault created by an older CLI.
	_, err = db.conn.Exec("INSERT INTO secrets(project_id,key_name,value_enc,created,updated) VALUES('p','Q_SECRET_KEY','not-decrypted','','')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.BuildEnv("p"); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("legacy guard: %v", err)
	}
}
