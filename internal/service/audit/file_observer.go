package audit

import (
	"context"
	"encoding/json"
	"os"

	"zerogravity-82/metrics/internal/model"
)

// FileObserver дописывает в файл сообщение события аудита в формате JSONL.
type FileObserver struct {
	path string
}

func NewFileObserver(path string) *FileObserver {
	return &FileObserver{path: path}
}

func (s *FileObserver) update(_ context.Context, log model.AuditLog) error {
	b, err := json.Marshal(log)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	_, err = f.Write(b)
	return err
}
