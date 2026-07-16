package db

import "testing"

func TestNewDBEmptyDSN(t *testing.T) {
	conn, err := NewDB("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Fatal("expected nil db for empty dsn")
	}
}

func TestNewDBInvalidDSN(t *testing.T) {
	_, err := NewDB("postgres://invalid:invalid@127.0.0.1:1/none?connect_timeout=1")
	if err == nil {
		t.Fatal("expected error for unreachable db")
	}
}
