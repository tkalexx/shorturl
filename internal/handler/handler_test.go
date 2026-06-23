package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func resetStorage() {
	urls = make(map[string]string)
	SetBaseURL("http://localhost:8080")
}

func TestGenerateID(t *testing.T) {
	id := generateID()
	if len(id) != 8 {
		t.Errorf("generateID() length = %d, want 8", len(id))
	}
}

func TestShortener(t *testing.T) {
	resetStorage()

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantPrefix  string
	}{
		{
			name:        "valid URL",
			contentType: "text/plain",
			body:        "https://ya.ru",
			wantStatus:  http.StatusCreated,
			wantPrefix:  "http://localhost:8080/",
		},
		{
			name:        "empty body",
			contentType: "text/plain",
			body:        "",
			wantStatus:  http.StatusBadRequest,
			wantPrefix:  "",
		},
		{
			name:        "wrong content-type",
			contentType: "application/json",
			body:        "https://ya.ru",
			wantStatus:  http.StatusBadRequest,
			wantPrefix:  "",
		},
		{
			name:        "invalid URL format",
			contentType: "text/plain",
			body:        "not-a-url",
			wantStatus:  http.StatusBadRequest,
			wantPrefix:  "",
		},
		{
			name:        "URL without scheme",
			contentType: "text/plain",
			body:        "ya.ru",
			wantStatus:  http.StatusBadRequest,
			wantPrefix:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rr := httptest.NewRecorder()

			shortener(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("shortener() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			if tt.wantPrefix != "" {
				got := rr.Body.String()
				if !strings.HasPrefix(got, tt.wantPrefix) {
					t.Errorf("shortener() body = %v, want prefix %v", got, tt.wantPrefix)
				}
			}
		})
	}
}

func TestShortenerDuplicate(t *testing.T) {
	resetStorage()

	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://praktikum.yandex.ru"))
	req1.Header.Set("Content-Type", "text/plain")
	rr1 := httptest.NewRecorder()
	shortener(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d", rr1.Code)
	}
	firstID := rr1.Body.String()

	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://praktikum.yandex.ru"))
	req2.Header.Set("Content-Type", "text/plain")
	rr2 := httptest.NewRecorder()
	shortener(rr2, req2)

	if rr2.Code != http.StatusCreated {
		t.Errorf("second request failed: %d", rr2.Code)
	}

	if rr2.Body.String() != firstID {
		t.Errorf("duplicate URL generated different ID: got %v, want %v", rr2.Body.String(), firstID)
	}
}

func TestExpander(t *testing.T) {
	resetStorage()

	// Добавляем тестовый URL
	testID := "abc123DEF"
	testURL := "https://praktikum.yandex.ru"
	urls[testID] = testURL

	tests := []struct {
		name       string
		id         string
		wantStatus int
		wantLoc    string
	}{
		{
			name:       "existing URL",
			id:         testID,
			wantStatus: http.StatusTemporaryRedirect,
			wantLoc:    testURL,
		},
		{
			name:       "non-existing URL",
			id:         "nonexistent",
			wantStatus: http.StatusNotFound,
			wantLoc:    "",
		},
		{
			name:       "empty path",
			id:         "",
			wantStatus: http.StatusBadRequest,
			wantLoc:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.id, nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.id)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			expander(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("expander() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			if tt.wantLoc != "" {
				loc := rr.Header().Get("Location")
				if loc != tt.wantLoc {
					t.Errorf("expander() Location header = %v, want %v", loc, tt.wantLoc)
				}
			}
		})
	}
}

func TestRouter(t *testing.T) {
	resetStorage()

	r := NewRouter()

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "POST valid",
			method:      http.MethodPost,
			path:        "/",
			body:        "https://ya.ru",
			contentType: "text/plain",
			wantStatus:  http.StatusCreated,
		},
		{
			name:       "GET non-existing id",
			method:     http.MethodGet,
			path:       "/nonexistent123",
			wantStatus: http.StatusNotFound,
		},
		{
			name:        "POST invalid content-type",
			method:      http.MethodPost,
			path:        "/",
			body:        "https://ya.ru",
			contentType: "application/json",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:       "PUT method not allowed",
			method:     http.MethodPut,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "DELETE method not allowed",
			method:     http.MethodDelete,
			path:       "/",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := strings.NewReader(tt.body)
			req := httptest.NewRequest(tt.method, tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("Router() %s: status = %v, want %v", tt.name, rr.Code, tt.wantStatus)
			}
		})
	}
}
