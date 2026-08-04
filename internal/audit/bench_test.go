package audit

import (
	"path/filepath"
	"testing"
)

func BenchmarkFileObserverNotify(b *testing.B) {
	path := filepath.Join(b.TempDir(), "audit.log")
	obs := NewFileObserver(path)
	event := Event{
		TS:     1234567890,
		Action: ActionShorten,
		UserID: "bench-user",
		URL:    "https://example.com/long/path/to/shorten",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := obs.Notify(event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditorNotify(b *testing.B) {
	auditor := NewAuditor()
	obs := NewFileObserver(filepath.Join(b.TempDir(), "audit.log"))
	auditor.Subscribe(obs)
	event := Event{
		TS:     1234567890,
		Action: ActionFollow,
		UserID: "bench-user",
		URL:    "https://example.com/long/path",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auditor.Notify(event)
	}
}
