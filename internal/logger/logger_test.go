package logger

import "testing"

func TestInitialize(t *testing.T) {
	if err := Initialize("info"); err != nil {
		t.Fatalf("Initialize info: %v", err)
	}
	if Log == nil {
		t.Fatal("Log is nil")
	}
	if err := Initialize("not-a-level"); err == nil {
		t.Fatal("expected error for invalid level")
	}
}
