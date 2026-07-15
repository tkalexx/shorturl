package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

const CookieName = "auth"

type contextKey struct{}

var userIDKey = contextKey{}

var ErrNoUserID = errors.New("user id not found")

// Manager подписывает и проверяет ауз куки
type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	if secret == "" {
		secret = "default-auth-secret"
	}
	return &Manager{secret: []byte(secret)}
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

func generateUserID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate user id")
	}
	return hex.EncodeToString(b)
}

func (m *Manager) setCookie(w http.ResponseWriter, userID string) {
	http.SetCookie(w, &http.Cookie{
		Name:  CookieName,
		Value: m.sign(userID),
		Path:  "/",
	})
}

// Middleware выдает, проверяет Cookie и кладет userID в контекст
// Cookie нет или подпись невалидна - создаем нового пользователя
// Cookie есть, но без ID пользователя - отдаем 401 ошибку
func (m *Manager) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := ""

		cookie, err := r.Cookie(CookieName)
		switch {
		case err == nil:
			if cookie.Value == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(cookie.Value, "|", 2)
			if parts[0] == "" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			id, verifyErr := m.verify(cookie.Value)
			if verifyErr != nil {
				userID = generateUserID()
				m.setCookie(w, userID)
			} else {
				userID = id
			}
		default:
			userID = generateUserID()
			m.setCookie(w, userID)
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WithUserID кладет идентификатор пользователя в контекст
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserIDFromContext возвращает идентификатор пользователя из контекста
func UserIDFromContext(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok || userID == "" {
		return "", ErrNoUserID
	}
	return userID, nil
}
