package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// CookieName - имя cookie с подписанным идентификатором пользователя.
const CookieName = "auth"

// AuthorizationMetadata — ключ metadata/header с токеном аутентификации для gRPC.
const AuthorizationMetadata = "authorization"

// Ошибки пакета auth.
var (
	ErrNoUserID       = errors.New("user id not found")
	ErrUnauthorized   = errors.New("unauthorized")
	ErrEmptySecret    = errors.New("auth secret is required")
	ErrSecretTooShort = errors.New("auth secret must be at least 16 characters")
)

type contextKey struct{}

var userIDKey = contextKey{}

// Manager подписывает и проверяет ауз куки
type Manager struct {
	secret []byte
}

func NewManager(secret string) (*Manager, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	if len(secret) < 16 {
		return nil, ErrSecretTooShort
	}
	return &Manager{secret: []byte(secret)}, nil
}

// sign возвращает значение куки вида "<userID>|<hex>"
func (m *Manager) sign(userID string) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(userID))
	return userID + "|" + hex.EncodeToString(mac.Sum(nil))
}

// verify разбирает подписанную куку и возвращает userID
func (m *Manager) verify(value string) (string, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrNoUserID
	}
	userID, sigHex := parts[0], parts[1]

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(userID))
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(sigHex)
	if err != nil {
		return "", err
	}
	if !hmac.Equal(expected, got) {
		return "", errors.New("invalid cookie signature")
	}
	return userID, nil
}

func generateUserID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate user id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// IssueToken возвращает подписанный токен вида "<userID>|<hex>" для cookie/metadata.
func (m *Manager) IssueToken(userID string) string {
	return m.sign(userID)
}

// UserIDFromToken проверяет токен и возвращает userID без создания нового пользователя.
func (m *Manager) UserIDFromToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", ErrNoUserID
	}
	parts := strings.SplitN(token, "|", 2)
	if parts[0] == "" {
		return "", ErrUnauthorized
	}
	return m.verify(token)
}

// Authenticate разбирает токен из cookie или gRPC metadata.
// Пустой/невалидный токен → новый пользователь (tokenChanged=true).
// Токен с пустым userID → ErrUnauthorized.
func (m *Manager) Authenticate(token string) (userID, issuedToken string, tokenChanged bool, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		userID, err = generateUserID()
		if err != nil {
			return "", "", false, err
		}
		return userID, m.sign(userID), true, nil
	}

	parts := strings.SplitN(token, "|", 2)
	if parts[0] == "" {
		return "", "", false, ErrUnauthorized
	}

	id, verifyErr := m.verify(token)
	if verifyErr != nil {
		userID, err = generateUserID()
		if err != nil {
			return "", "", false, err
		}
		return userID, m.sign(userID), true, nil
	}
	return id, token, false, nil
}

// Middleware выдает, проверяет Cookie и кладет userID в контекст
// Cookie нет или подпись невалидна - создаем нового пользователя
// Cookie есть, но без ID пользователя - отдаем 401 ошибку
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if cookie, err := r.Cookie(CookieName); err == nil {
			if cookie.Value == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			token = cookie.Value
		}

		userID, issuedToken, tokenChanged, err := m.Authenticate(token)
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		if tokenChanged {
			http.SetCookie(w, &http.Cookie{
				Name:  CookieName,
				Value: issuedToken,
				Path:  "/",
			})
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithUserID кладет идентификатор пользователя в контекст
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext возвращает идентификатор пользователя из контекста.
// ok == false, если идентификатор отсутствует или пустой.
func UserIDFromContext(ctx context.Context) (userID string, ok bool) {
	userID, ok = ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		return "", false
	}
	return userID, true
}
