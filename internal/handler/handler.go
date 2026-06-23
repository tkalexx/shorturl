package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
)

var urls = make(map[string]string)
var baseURL string // будет установлен из main

// SetBaseURL устанавливает базовый URL из конфига
func SetBaseURL(url string) {
	baseURL = url
}

type ShortenRequest struct {
	URL string `json:"url"`
}

type ShortenResponse struct {
	Result string `json:"result"`
}

// generateID генерирует уникальный ID
func generateID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random ID")
	}
	return base64.URLEncoding.EncodeToString(b)[:8]
}

// shortener возвращает сокращённый URL
func shortener(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(string(b))
	if originalURL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	for id, existingURL := range urls {
		if existingURL == originalURL {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(baseURL + "/" + id))
			return
		}
	}

	var id string
	for {
		id = generateID()
		if _, exists := urls[id]; !exists {
			break
		}
	}

	urls[id] = originalURL

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(baseURL + "/" + id))
}

// expander возвращает оригинальный URL
func expander(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "URL ID is required", http.StatusBadRequest)
		return
	}

	originalURL, ok := urls[id]
	if !ok {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// shortenJSON - новый handler для POST /api/shorten
func shortenJSON(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	originalURL := strings.TrimSpace(req.URL)
	if originalURL == "" {
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	if id, exists := findURL(originalURL); exists {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ShortenResponse{Result: baseURL + "/" + id})
		return
	}

	var id string
	for {
		id = generateID()
		if _, exists := urls[id]; !exists {
			break
		}
	}

	urls[id] = originalURL

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ShortenResponse{Result: baseURL + "/" + id})
}

func findURL(originalURL string) (string, bool) {
	for id, existingURL := range urls {
		if existingURL == originalURL {
			return id, true
		}
	}
	return "", false
}

func NewRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/", shortener)
	r.Get("/{id}", expander)
	r.Post("/api/shorten", shortenJSON)
	return r
}
