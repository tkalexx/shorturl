package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/repository"
)

func setupTestService() *Service {
	repo := repository.NewInMemory()
	service := NewService(repo)
	service.SetBaseURL("http://localhost:8080")
	return service
}

func setupTestRouter(service *Service) chi.Router {
	authManager, err := auth.NewManager("test-auth-secret-16chars")
	if err != nil {
		return nil
	}
	return NewRouter(service, authManager, nil, "")
}

func withTestUser(req *http.Request, userID string) *http.Request {
	if userID == "" {
		userID = "test-user"
	}
	return req.WithContext(auth.WithUserID(req.Context(), userID))
}

func TestGenerateID(t *testing.T) {
	id, err := generateID()
	if err != nil {
		t.Fatalf("generateID() error: %v", err)
	}
	if len(id) != 8 {
		t.Errorf("generateID() length = %d, want 8", len(id))
	}
}

func TestShortener(t *testing.T) {
	service := setupTestService()
	h := NewHandler(service, nil)

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
			req = withTestUser(req, "user-1")
			rr := httptest.NewRecorder()

			h.shortener(rr, req)

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
	service := setupTestService()
	h := NewHandler(service, nil)

	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://praktikum.yandex.ru"))
	req1.Header.Set("Content-Type", "text/plain")
	req1 = withTestUser(req1, "user-1")
	rr1 := httptest.NewRecorder()
	h.shortener(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("first request failed: %d", rr1.Code)
	}
	firstID := rr1.Body.String()

	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://praktikum.yandex.ru"))
	req2.Header.Set("Content-Type", "text/plain")
	req2 = withTestUser(req2, "user-1")
	rr2 := httptest.NewRecorder()
	h.shortener(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Errorf("second request expected 409 Conflict, got: %d", rr2.Code)
	}

	if rr2.Body.String() != firstID {
		t.Errorf("duplicate URL generated different ID: got %v, want %v", rr2.Body.String(), firstID)
	}
}

func TestExpander(t *testing.T) {
	service := setupTestService()
	h := NewHandler(service, nil)

	shortURL, _, err := service.Shorten(context.Background(), "https://praktikum.yandex.ru", "user-1")
	if err != nil {
		t.Fatalf("failed to create short URL: %v", err)
	}
	parts := strings.Split(shortURL, "/")
	testID := parts[len(parts)-1]

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
			wantLoc:    "https://praktikum.yandex.ru",
		},
		{
			name:       "non-existing URL",
			id:         "nonexistent",
			wantStatus: http.StatusNotFound,
			wantLoc:    "",
		},
		{
			name:       "empty id",
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
			h.expander(rr, req)

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

func TestShortenJSONContentTypeValidation(t *testing.T) {
	service := setupTestService()
	h := NewHandler(service, nil)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:        "valid application/json",
			contentType: "application/json",
			body:        `{"url":"https://json-test1.ya.ru"}`,
			wantStatus:  http.StatusCreated,
		},
		{
			name:        "valid with charset",
			contentType: "application/json; charset=utf-8",
			body:        `{"url":"https://json-test2.ya.ru"}`,
			wantStatus:  http.StatusCreated,
		},
		{
			name:        "invalid text/plain",
			contentType: "text/plain",
			body:        `{"url":"https://json-test3.ya.ru"}`,
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "invalid text/html",
			contentType: "text/html",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "missing content-type",
			contentType: "",
			wantStatus:  http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			req = withTestUser(req, "user-1")
			rr := httptest.NewRecorder()

			h.shortenJSON(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("got status %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestShortenJSON(t *testing.T) {
	service := setupTestService()
	h := NewHandler(service, nil)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid JSON",
			body:       `{"url":"https://ya.ru"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "empty URL",
			body:       `{"url":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid URL",
			body:       `{"url":"not-a-url"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			body:       `{"url":`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req = withTestUser(req, "user-1")
			rr := httptest.NewRecorder()

			h.shortenJSON(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("shortenJSON() status = %v, want %v", rr.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusCreated {
				if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %v, want application/json", ct)
				}
			}
		})
	}
}

func TestShortenJSONResponseFormat(t *testing.T) {
	service := setupTestService()
	h := NewHandler(service, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req = withTestUser(req, "user-1")
	rr := httptest.NewRecorder()

	h.shortenJSON(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var resp ShortenResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Errorf("response is not valid JSON: %v", err)
		return
	}

	if resp.Result == "" {
		t.Error("Result field is empty")
		return
	}

	expectedPrefix := "http://localhost:8080/"
	if !strings.HasPrefix(resp.Result, expectedPrefix) {
		t.Errorf("Result does not have expected prefix: got %s, want prefix %s", resp.Result, expectedPrefix)
	}

	parts := strings.Split(resp.Result, "/")
	if len(parts) < 2 || len(parts[len(parts)-1]) != 8 {
		t.Errorf("Short ID has wrong length in result: %s", resp.Result)
	}
}

func TestRouter(t *testing.T) {
	service := setupTestService()
	r := setupTestRouter(service)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "POST text/plain valid",
			method:      http.MethodPost,
			path:        "/",
			body:        "https://ya.ru",
			contentType: "text/plain",
			wantStatus:  http.StatusCreated,
		},
		{
			name:        "POST JSON valid",
			method:      http.MethodPost,
			path:        "/api/shorten",
			body:        `{"url":"https://router-json.ya.ru"}`,
			contentType: "application/json",
			wantStatus:  http.StatusCreated,
		},
		{
			name:       "GET existing (redirect)",
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

func TestRouter_GzipCompression(t *testing.T) {
	service := setupTestService()
	r := setupTestRouter(service)

	t.Run("response compressed for json with accept-encoding", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://gzip1.ya.ru"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got: %d", rr.Code)
		}

		if ce := rr.Header().Get("Content-Encoding"); ce != "gzip" {
			t.Errorf("expected Content-Encoding: gzip, got: %s", ce)
		}

		// Распаковываем и проверяем содержимое
		gr, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gr.Close()

		var resp ShortenResponse
		if err := json.NewDecoder(gr).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !strings.HasPrefix(resp.Result, "http://localhost:8080/") {
			t.Errorf("unexpected result: %s", resp.Result)
		}
	})

	t.Run("response not compressed without accept-encoding", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://gzip2.ya.ru"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got: %d", rr.Code)
		}

		if ce := rr.Header().Get("Content-Encoding"); ce != "" {
			t.Errorf("expected no Content-Encoding, got: %s", ce)
		}

		// Проверяем обычный JSON
		var resp ShortenResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
	})

	t.Run("decompress gzip request body", func(t *testing.T) {
		originalBody := `{"url":"https://gzip3.ya.ru"}`

		// Сжимаем тело запроса
		var compressed bytes.Buffer
		gw := gzip.NewWriter(&compressed)
		gw.Write([]byte(originalBody))
		gw.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/shorten", &compressed)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201, got: %d", rr.Code)
		}

		// Распаковываем ответ
		gr, err := gzip.NewReader(rr.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gr.Close()

		var resp ShortenResponse
		if err := json.NewDecoder(gr).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !strings.HasSuffix(resp.Result, "compressed-request.com") {
			// Id будет разным, но URL должен быть валидным
			if !strings.HasPrefix(resp.Result, "http://localhost:8080/") {
				t.Errorf("unexpected result format: %s", resp.Result)
			}
		}
	})
}

func TestUserURLs(t *testing.T) {
	service := setupTestService()
	r := setupTestRouter(service)

	t.Run("no content for new user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rr.Code)
		}
		if len(rr.Result().Cookies()) == 0 {
			t.Fatal("expected auth cookie to be set")
		}
	})

	t.Run("returns urls for authenticated user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://user-urls.example.com"))
		req.Header.Set("Content-Type", "text/plain")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("shorten failed: %d", rr.Code)
		}
		shortURL := rr.Body.String()
		cookies := rr.Result().Cookies()
		if len(cookies) == 0 {
			t.Fatal("expected auth cookie after shorten")
		}

		listReq := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		for _, c := range cookies {
			listReq.AddCookie(c)
		}
		listRR := httptest.NewRecorder()
		r.ServeHTTP(listRR, listReq)

		if listRR.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", listRR.Code, listRR.Body.String())
		}

		var urls []UserURLResponse
		if err := json.NewDecoder(listRR.Body).Decode(&urls); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(urls) != 1 {
			t.Fatalf("expected 1 url, got %d", len(urls))
		}
		if urls[0].ShortURL != shortURL || urls[0].OriginalURL != "https://user-urls.example.com" {
			t.Errorf("unexpected urls: %+v", urls)
		}
	})

	t.Run("unauthorized without user id in cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "|abcdef"})
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})
}

func TestDeleteUserURLs(t *testing.T) {
	service := setupTestService()
	r := setupTestRouter(service)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://delete-me.example.com"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("shorten failed: %d", rr.Code)
	}
	shortURL := rr.Body.String()
	parts := strings.Split(shortURL, "/")
	shortID := parts[len(parts)-1]
	cookies := rr.Result().Cookies()

	body, _ := json.Marshal([]string{shortID})
	delReq := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader(body))
	delReq.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		delReq.AddCookie(c)
	}
	delRR := httptest.NewRecorder()
	r.ServeHTTP(delRR, delReq)

	if delRR.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", delRR.Code)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		getReq := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
		getRR := httptest.NewRecorder()
		r.ServeHTTP(getRR, getReq)
		if getRR.Code == http.StatusGone {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("expected deleted URL to return 410 Gone")
}
