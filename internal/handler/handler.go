package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tkalexx/shorturl.git/internal/gzip"
	"github.com/tkalexx/shorturl.git/internal/logger"
	"github.com/tkalexx/shorturl.git/internal/repository"
)

var (
	ErrInvalidURL  = errors.New("invalid URL format")
	ErrEmptyURL    = errors.New("URL cannot be empty")
	ErrURLNotFound = errors.New("URL not found")
)

type Service struct {
	repo    repository.Repository
	baseURL string
}

func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetBaseURL(url string) {
	s.baseURL = strings.TrimSuffix(url, "/")
}

func (s *Service) Shorten(originalURL string) (string, bool, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "", false, ErrEmptyURL
	}

	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", false, ErrInvalidURL
	}

	if id, found := s.repo.FindByURL(originalURL); found {
		return s.baseURL + "/" + id, true, nil
	}

	var id string
	for {
		id = generateID()
		if _, exists := s.repo.Get(id); !exists {
			break
		}
	}

	if err := s.repo.Save(id, originalURL); err != nil {
		return "", false, err
	}

	return s.baseURL + "/" + id, false, nil
}

func (s *Service) Get(id string) (string, error) {
	if id == "" {
		return "", ErrEmptyURL
	}
	if url, ok := s.repo.Get(id); ok {
		return url, nil
	}
	return "", ErrURLNotFound
}

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
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
func (h *Handler) shortener(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}

	shortURL, _, err := h.service.Shorten(string(b))
	if err != nil {
		mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(shortURL))
}

// expander возвращает оригинальный URL
func (h *Handler) expander(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	originalURL, err := h.service.Get(id)
	if err != nil {
		if errors.Is(err, ErrURLNotFound) {
			http.Error(w, "URL not found", http.StatusNotFound)
			return
		}
		http.Error(w, "URL ID is required", http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", originalURL)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

func mapError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEmptyURL):
		http.Error(w, "URL cannot be empty", http.StatusBadRequest)
	case errors.Is(err, ErrInvalidURL):
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
	default:
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

func NewRouter(service *Service) chi.Router {
	r := chi.NewRouter()
	r.Use(logger.LoggingMiddleware)
	r.Use(gzip.Middleware)

	h := NewHandler(service)

	r.Post("/", h.shortener)
	r.Get("/{id}", h.expander)
	r.Post("/api/shorten", h.shortenJSON)

	return r
}

// shortenJSON - новый handler для POST /api/shorten
func (h *Handler) shortenJSON(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	shortURL, _, err := h.service.Shorten(req.URL)
	if err != nil {
		mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	resp := ShortenResponse{Result: shortURL}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
