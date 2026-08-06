package audit

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type recordingObserver struct {
	mu     sync.Mutex
	events []Event
}

func (r *recordingObserver) Notify(event Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func waitForEvents(t *testing.T, obs *recordingObserver, n int) []Event {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		obs.mu.Lock()
		if len(obs.events) >= n {
			events := append([]Event(nil), obs.events...)
			obs.mu.Unlock()
			return events
		}
		obs.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	t.Fatalf("timed out waiting for %d events, got %d", n, len(obs.events))
	return nil
}

func TestAuditorNotifiesSubscribers(t *testing.T) {
	auditor := NewAuditor()
	defer auditor.Close()

	obs := &recordingObserver{}
	auditor.Subscribe(obs)

	auditor.LogShorten("user-1", "https://example.com/long")
	auditor.LogFollow("", "https://example.com/long")

	events := waitForEvents(t, obs, 2)
	if events[0].Action != ActionShorten || events[0].UserID != "user-1" {
		t.Fatalf("unexpected shorten event: %+v", events[0])
	}
	if events[1].Action != ActionFollow || events[1].URL != "https://example.com/long" {
		t.Fatalf("unexpected follow event: %+v", events[1])
	}
	if events[0].TS == 0 || events[1].TS == 0 {
		t.Fatal("expected non-zero timestamps")
	}
}

func TestFileObserverAppendsJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	obs := NewFileObserver(path)

	event := Event{
		TS:     time.Now().Unix(),
		Action: ActionShorten,
		UserID: "u1",
		URL:    "https://ya.ru",
	}
	if err := obs.Notify(event); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if err := obs.Notify(event); err != nil {
		t.Fatalf("Notify second: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := 0
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var got Event
		if err := json.Unmarshal(line, &got); err != nil {
			t.Fatalf("unmarshal: %v line=%q", err, line)
		}
		if got.Action != ActionShorten || got.URL != "https://ya.ru" {
			t.Fatalf("unexpected event: %+v", got)
		}
		lines++
	}
	if lines != 2 {
		t.Fatalf("expected 2 lines, got %d", lines)
	}
}

func TestHTTPObserverPostsJSON(t *testing.T) {
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method=%s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type=%s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	obs := NewHTTPObserver(srv.URL)
	event := Event{TS: 42, Action: ActionFollow, UserID: "u2", URL: "https://go.dev"}
	if err := obs.Notify(event); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if got != event {
		t.Fatalf("got %+v want %+v", got, event)
	}
}

func TestBuildFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.log")
	auditor := BuildFromConfig(path, "")
	defer auditor.Close()

	auditor.LogShorten("u", "https://example.com")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected audit file content")
	}

	empty := BuildFromConfig("", "")
	defer empty.Close()
	empty.LogShorten("u", "https://example.com")
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
