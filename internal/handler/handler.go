package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tkalexx/shorturl.git/internal/auth"
	"github.com/tkalexx/shorturl.git/internal/gzip"
	"github.com/tkalexx/shorturl.git/internal/logger"
	"github.com/tkalexx/shorturl.git/internal/repository"
	"go.uber.org/zap"
)

var (
	ErrInvalidURL  = errors.New("invalid URL format")
	ErrEmptyURL    = errors.New("URL cannot be empty")
	ErrURLNotFound = errors.New("URL not found")
	ErrURLDeleted  = errors.New("URL deleted")
)

type Service struct {
	repo     repository.Repository
	baseURL  string
	deleteCh chan deleteTask
	done     chan struct{}
}

func NewService(repo repository.Repository) *Service {
	s := &Service{
		repo:     repo,
		deleteCh: make(chan deleteTask, 1024),
		done:     make(chan struct{}),
	}
	s.startDeleteWorker()
	return s
}

type BatchItem struct {
	CorrelationID string `json:"correlation_id"`
	OriginalURL   string `json:"original_url"`
}

type BatchResponseItem struct {
	CorrelationID string `json:"correlation_id"`
	ShortURL      string `json:"short_url"`
}

type UserURLResponse struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// Ping проверяет соединение с БД
func (s *Service) Ping(ctx context.Context) error {
	if s.repo == nil {
		return fmt.Errorf("repository not initialized")
	}
	return s.repo.Ping(ctx)
}

func (s *Service) SetBaseURL(url string) {
	s.baseURL = strings.TrimSuffix(url, "/")
}

func (s *Service) Shorten(ctx context.Context, originalURL, userID string) (string, bool, error) {
	originalURL = strings.TrimSpace(originalURL)
	if originalURL == "" {
		return "", false, ErrEmptyURL
	}

	parsedURL, err := url.ParseRequestURI(originalURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", false, ErrInvalidURL
	}

	id, err := generateID()
	if err != nil {
		return "", false, err
	}

	err = s.repo.Save(ctx, id, originalURL, userID)
	if err != nil {
		if errors.Is(err, repository.ErrURLExists) {
			existingID, found := s.repo.FindByURL(ctx, originalURL)
			if !found {
				return "", false, fmt.Errorf("inconsistent state: URL exists but not found")
			}
			return s.baseURL + "/" + existingID, true, nil
		}
		return "", false, err
	}

	return s.baseURL + "/" + id, false, nil
}

func (s *Service) ShortenBatch(ctx context.Context, items []BatchItem, userID string) ([]BatchResponseItem, error) {
	if len(items) == 0 {
		return []BatchResponseItem{}, nil
	}

	pairs := make([]repository.URLPair, len(items))
	for i, item := range items {
		originalURL := strings.TrimSpace(item.OriginalURL)
		if originalURL == "" {
			return nil, ErrEmptyURL
		}
		if _, err := url.ParseRequestURI(originalURL); err != nil {
			return nil, ErrInvalidURL
		}

		id, err := generateID()
		if err != nil {
			return nil, err
		}

		pairs[i] = repository.URLPair{
			ID:     id,
			URL:    originalURL,
			UserID: userID,
		}
	}

	mapping, err := s.repo.SaveBatch(ctx, pairs)
	if err != nil {
		return nil, err
	}

	result := make([]BatchResponseItem, len(items))
	for i, item := range items {
		result[i] = BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      s.baseURL + "/" + mapping[item.OriginalURL],
		}
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id string) (string, error) {
	if id == "" {
		return "", ErrEmptyURL
	}
	url, deleted, found := s.repo.Get(ctx, id)
	if !found {
		return "", ErrURLNotFound
	}
	if deleted {
		return "", ErrURLDeleted
	}
	return url, nil
}

func (s *Service) GetUserURLs(ctx context.Context, userID string) ([]UserURLResponse, error) {
	urls, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]UserURLResponse, 0, len(urls))
	for _, u := range urls {
		result = append(result, UserURLResponse{
			ShortURL:    s.baseURL + "/" + u.ID,
			OriginalURL: u.OriginalURL,
		})
	}
	return result, nil
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
func generateID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b)[:8], nil
}

func userIDFromRequest(r *http.Request) (string, error) {
	return auth.UserIDFromContext(r.Context())
}

// ping handler для GET /ping
func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.service.Ping(ctx); err != nil {
		logger.Log.Error("Database ping failed", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// shortener возвращает сокращённый URL
func (h *Handler) shortener(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") {
		http.Error(w, "Content-Type must be text/plain", http.StatusBadRequest)
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	b, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Cannot read request body", http.StatusBadRequest)
		return
	}

	shortURL, exists, err := h.service.Shorten(r.Context(), string(b), userID)
	if err != nil {
		mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	if exists {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	w.Write([]byte(shortURL))
}

func (h *Handler) shortenBatch(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req []BatchItem
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("[]"))
		return
	}

	resp, err := h.service.ShortenBatch(r.Context(), req, userID)
	if err != nil {
		mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// userURLs возвращает все URL, сокращённые текущим пользователем
func (h *Handler) userURLs(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	urls, err := h.service.GetUserURLs(r.Context(), userID)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if len(urls) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(urls); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// deleteUserURLs асинхронно удаляет URL пользователя
func (h *Handler) deleteUserURLs(w http.ResponseWriter, r *http.Request) {
	userID, err := userIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var ids []string
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	h.service.DeleteUserURLs(userID, ids)
	w.WriteHeader(http.StatusAccepted)
}

// expander возвращает оригинальный URL
func (h *Handler) expander(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	originalURL, err := h.service.Get(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrURLDeleted):
			w.WriteHeader(http.StatusGone)
		case errors.Is(err, ErrURLNotFound):
			http.Error(w, "URL not found", http.StatusNotFound)
		default:
			http.Error(w, "URL ID is required", http.StatusBadRequest)
		}
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

func NewRouter(service *Service, authManager *auth.Manager) chi.Router {
	r := chi.NewRouter()
	r.Use(logger.LoggingMiddleware)
	r.Use(gzip.Middleware)

	h := NewHandler(service)

	r.Get("/ping", h.ping)

	r.Group(func(r chi.Router) {
		r.Use(authManager.Middleware)
		r.Post("/", h.shortener)
		r.Post("/api/shorten", h.shortenJSON)
		r.Post("/api/shorten/batch", h.shortenBatch)
		r.Get("/api/user/urls", h.userURLs)
		r.Delete("/api/user/urls", h.deleteUserURLs)
	})

	r.Get("/{id}", h.expander)

	return r
}

// shortenJSON - новый handler для POST /api/shorten
func (h *Handler) shortenJSON(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		http.Error(w, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	shortURL, exists, err := h.service.Shorten(r.Context(), req.URL, userID)
	if err != nil {
		mapError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if exists {
		w.WriteHeader(http.StatusConflict)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	resp := ShortenResponse{Result: shortURL}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
