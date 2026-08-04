package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/tkalexx/shorturl.git/internal/logger"
	"go.uber.org/zap"
)

const ActionShorten = "shorten"

const ActionFollow = "follow"

// Event - событие аудита
type Event struct {
	TS     int64  `json:"ts"`
	Action string `json:"action"`
	UserID string `json:"user_id"`
	URL    string `json:"url"`
}

// Observer принимает уведомления о событиях аудита
type Observer interface {
	Notify(event Event) error
}

// Auditor - субъект, рассылающий события всем подписчикам
type Auditor struct {
	mu        sync.RWMutex
	observers []Observer
}

// NewAuditor создаёт пустой Auditor без подписчиков
func NewAuditor() *Auditor {
	return &Auditor{}
}

// Subscribe добавляет приёмник аудита.
func (a *Auditor) Subscribe(o Observer) {
	if o == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.observers = append(a.observers, o)
}

// Notify уведомляет всех подписчиков о событии.
func (a *Auditor) Notify(event Event) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, o := range a.observers {
		if err := o.Notify(event); err != nil {
			logger.Log.Error("audit notify failed", zap.Error(err))
		}
	}
}

// LogShorten формирует и рассылает событие создания ссылки.
func (a *Auditor) LogShorten(userID, originalURL string) {
	a.Notify(Event{
		TS:     time.Now().Unix(),
		Action: ActionShorten,
		UserID: userID,
		URL:    originalURL,
	})
}

// LogFollow формирует и рассылает событие перехода по ссылке.
func (a *Auditor) LogFollow(userID, originalURL string) {
	a.Notify(Event{
		TS:     time.Now().Unix(),
		Action: ActionFollow,
		UserID: userID,
		URL:    originalURL,
	})
}

// FileObserver пишет события аудита в файл (по одной JSON-строке).
type FileObserver struct {
	path string
	mu   sync.Mutex
	file *os.File
	buf  []byte
}

// NewFileObserver создаёт приёмник, пишущий в path.
func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path, buf: make([]byte, 0, 256)}
}

func (f *FileObserver) openLocked() error {
	if f.file != nil {
		return nil
	}
	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open audit file: %w", err)
	}
	f.file = file
	return nil
}

// Notify добавляет событие в конец файла на новой строке.
func (f *FileObserver) Notify(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.openLocked(); err != nil {
		return err
	}

	f.buf = f.buf[:0]
	f.buf = append(f.buf, data...)
	f.buf = append(f.buf, '\n')

	if _, err := f.file.Write(f.buf); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}

// HTTPObserver отправляет события аудита POST-запросом на удалённый URL.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт приёмник для удалённого сервера.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Notify отправляет событие методом POST.
func (h *HTTPObserver) Notify(event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create audit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("send audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("audit server returned status %d", resp.StatusCode)
	}
	return nil
}

// BuildFromConfig создаёт Auditor и подключает приёмники по путям конфигурации.
// Пустые auditFile / auditURL означают, что соответствующий приёмник отключён.
func BuildFromConfig(auditFile, auditURL string) *Auditor {
	auditor := NewAuditor()
	if auditFile != "" {
		auditor.Subscribe(NewFileObserver(auditFile))
	}
	if auditURL != "" {
		auditor.Subscribe(NewHTTPObserver(auditURL))
	}
	return auditor
}
