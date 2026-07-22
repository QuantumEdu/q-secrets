package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatchDBFile_DetectsWrite(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create the file first
	if err := os.WriteFile(dbPath, []byte("initial"), 0644); err != nil {
		t.Fatalf("creating test db file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	changed, err := watchDBFile(ctx, dbPath)
	if err != nil {
		t.Fatalf("watchDBFile failed: %v", err)
	}

	// Write to the file to trigger a change
	if err := os.WriteFile(dbPath, []byte("updated"), 0644); err != nil {
		t.Fatalf("writing to test db file: %v", err)
	}

	select {
	case <-changed:
		// Success — change was detected
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for file change event")
	}
}

func TestWatchDBFile_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nonexistent.db")

	ctx := context.Background()
	_, err := watchDBFile(ctx, dbPath)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestWatchDBFile_CancelContext(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	if err := os.WriteFile(dbPath, []byte("data"), 0644); err != nil {
		t.Fatalf("creating test db file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	changed, err := watchDBFile(ctx, dbPath)
	if err != nil {
		t.Fatalf("watchDBFile failed: %v", err)
	}

	// Cancel immediately
	cancel()

	// Verify the goroutine exits cleanly (no panic)
	select {
	case <-changed:
		// Might get a lingering event — that's fine
	case <-time.After(2 * time.Second):
		// No event — also fine, the watcher shut down
	}
}
