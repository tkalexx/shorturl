package handler

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var urls = make(map[string]string)

func generateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:8]
}

func Router(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		shortener(w, r)
	case http.MethodGet:
		expander(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func shortener(w http.ResponseWriter, r *http.Request) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}
	originalURL := strings.TrimSpace(string(b))

	// Улучшенная валидация URL
	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		http.Error(w, "Bad URL given", http.StatusBadRequest)
		return
	}

	for id, existingURL := range urls {
		if existingURL == originalURL {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte("http://localhost:8080/" + id))
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
	w.Write([]byte("http://localhost:8080/" + id))
}

func expander(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) < 2 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad ID given"))
		return
	}
	param := r.URL.Path[1:]

	target, ok := urls[param]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusTemporaryRedirect)
}
