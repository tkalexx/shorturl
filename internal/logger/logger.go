package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Log синглтон логера
var Log *zap.Logger = zap.NewNop()

// Initialize инициализирует логер с указанным уровнем.
func Initialize(level string) error {
	// преобразуем текстовый уровень логирования в zap.AtomicLevel
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}
	// создаём новую конфигурацию логера
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	// создаём логер на основе конфигурации
	zl, err := cfg.Build()
	if err != nil {
		return err
	}
	Log = zl
	return nil
}

// responseWriter обертка для перехвата статуса и размера ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader сохраняет код ответа и делегирует запись исходному ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write считает размер ответа и делегирует запись исходному ResponseWriter.
func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// LoggingMiddleware логирует URI, метод, длительность, статус и размер ответа.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			size:           0,
		}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		Log.Info("request handled",
			zap.String("uri", r.URL.Path),
			zap.String("method", r.Method),
			zap.Duration("duration", duration),
			zap.Int("status", wrapped.statusCode),
			zap.Int("size", wrapped.size),
		)
	})
}
