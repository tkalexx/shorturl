package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/handler"
	"github.com/tkalexx/shorturl.git/internal/repository"
)

func exampleRouter() http.Handler {
	repo := repository.NewInMemory()
	service := handler.NewService(repo)
	service.SetBaseURL("http://localhost:8080")

	authManager, err := auth.NewManager("example-auth-secret")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return handler.NewRouter(service, authManager, nil)
}

// Example_shortenPlain демонстрирует POST / — сокращение URL в text/plain.
func Example_shortenPlain() {
	router := exampleRouter()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(strings.HasPrefix(rec.Body.String(), "http://localhost:8080/"))
	// Output:
	// 201
	// true
}

// Example_shortenJSON демонстрирует POST /api/shorten.
func Example_shortenJSON() {
	router := exampleRouter()

	body := `{"url":"https://practicum.yandex.ru"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	var resp handler.ShortenResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	fmt.Println(rec.Code)
	fmt.Println(strings.HasPrefix(resp.Result, "http://localhost:8080/"))
	// Output:
	// 201
	// true
}

// Example_expand демонстрирует GET /{id} — редирект на оригинальный URL.
func Example_expand() {
	router := exampleRouter()

	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	createReq.Header.Set("Content-Type", "text/plain")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	shortURL := strings.TrimSpace(createRec.Body.String())
	id := shortURL[strings.LastIndex(shortURL, "/")+1:]

	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Header().Get("Location"))
	// Output:
	// 307
	// https://practicum.yandex.ru
}

// Example_shortenBatch демонстрирует POST /api/shorten/batch.
func Example_shortenBatch() {
	router := exampleRouter()

	payload := []handler.BatchItem{
		{CorrelationID: "1", OriginalURL: "https://practicum.yandex.ru"},
		{CorrelationID: "2", OriginalURL: "https://ya.ru"},
	}
	data, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/shorten/batch", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Body)
	var resp []handler.BatchResponseItem
	_ = json.Unmarshal(body, &resp)

	fmt.Println(rec.Code)
	fmt.Println(len(resp))
	// Output:
	// 201
	// 2
}

// Example_userURLs демонстрирует GET /api/user/urls.
func Example_userURLs() {
	router := exampleRouter()

	createReq := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"https://practicum.yandex.ru"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	cookie := createRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var urls []handler.UserURLResponse
	_ = json.NewDecoder(rec.Body).Decode(&urls)

	fmt.Println(rec.Code)
	fmt.Println(len(urls) > 0)
	// Output:
	// 200
	// true
}

// Example_deleteUserURLs демонстрирует DELETE /api/user/urls.
func Example_deleteUserURLs() {
	router := exampleRouter()

	createReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("https://practicum.yandex.ru"))
	createReq.Header.Set("Content-Type", "text/plain")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)

	cookie := createRec.Result().Cookies()[0]
	shortURL := strings.TrimSpace(createRec.Body.String())
	id := shortURL[strings.LastIndex(shortURL, "/")+1:]

	payload, _ := json.Marshal([]string{id})
	req := httptest.NewRequest(http.MethodDelete, "/api/user/urls", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	// Output:
	// 202
}
