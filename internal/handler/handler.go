package handler

import (
	"crypto/rand"
	"encoding/base64"
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
	if r.Header.Get("Content-Type") != "text/plain" {
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
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		http.Error(w, "URL ID is required", http.StatusBadRequest)
		return
	}

	originalURL, ok := urls[path]
	if !ok {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func NewRouter() chi.Router {
	r := chi.NewRouter()
	r.Post("/", shortener)
	r.Get("/{id}", expander)
	return r
}
