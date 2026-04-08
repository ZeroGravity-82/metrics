package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"zerogravity-82/metrics/internal/model"
)

// FileObserver дописывает в файл сообщение события аудита в формате JSONL.
type FileObserver struct {
	file   *os.File
	closed bool
	mu     sync.Mutex
}

// NewFileObserver создает FileObserver с файлом для записи по указанному пути.
func NewFileObserver(path string) (*FileObserver, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for audit observer: %w", err)
	}
	return &FileObserver{file: f}, nil
}

func (o *FileObserver) update(_ context.Context, log model.AuditLog) error {
	b, err := json.Marshal(log)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		_, err = o.file.Write(b)
	}
	return err
}

func (o *FileObserver) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return nil
	}
	o.closed = true
	err := o.file.Close()
	return err
}
