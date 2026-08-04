package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkSign(b *testing.B) {
	m, err := NewManager("benchmark-secret-16")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.sign("user-id-1234567890")
	}
}

func BenchmarkVerify(b *testing.B) {
	m, err := NewManager("benchmark-secret-16")
	if err != nil {
		b.Fatal(err)
	}
	value := m.sign("user-id-1234567890")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := m.verify(value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMiddleware(b *testing.B) {
	m, err := NewManager("benchmark-secret-16")
	if err != nil {
		b.Fatal(err)
	}
	h := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	cookieValue := m.sign("user-id-1234567890")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: CookieName, Value: cookieValue})
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
	}
}
