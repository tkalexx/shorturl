package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tkalexx/shorturl.git/internal/auth"
)

func setupTestRouterWithSubnet(service *Service, trustedSubnet string) chi.Router {
	authManager, err := auth.NewManager("test-auth-secret-16chars")
	if err != nil {
		return nil
	}
	return NewRouter(service, authManager, nil, trustedSubnet)
}

func TestInternalStatsAccess(t *testing.T) {
	service := setupTestService()
	router := setupTestRouterWithSubnet(service, "192.168.0.0/16")

	t.Run("forbidden without trusted subnet header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status=%d, want 403", rr.Code)
		}
	})

	t.Run("forbidden for foreign subnet", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		req.Header.Set("X-Real-IP", "10.0.0.1")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status=%d, want 403", rr.Code)
		}
	})

	t.Run("ok for trusted ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://ya.ru"))
		req.Header.Set("Content-Type", "text/plain")
		createRR := httptest.NewRecorder()
		router.ServeHTTP(createRR, req)
		if createRR.Code != http.StatusCreated {
			t.Fatalf("shorten status=%d", createRR.Code)
		}

		statsReq := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
		statsReq.Header.Set("X-Real-IP", "192.168.1.10")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, statsReq)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}

		var got StatsResponse
		if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.URLs < 1 || got.Users < 1 {
			t.Fatalf("stats=%+v, want urls>=1 users>=1", got)
		}
	})
}

func TestInternalStatsEmptySubnetForbidden(t *testing.T) {
	service := setupTestService()
	router := setupTestRouterWithSubnet(service, "")

	req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
	req.Header.Set("X-Real-IP", "127.0.0.1")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", rr.Code)
	}
}
