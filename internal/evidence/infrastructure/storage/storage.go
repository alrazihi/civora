package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidFilename    = errors.New("invalid filename")
	ErrUnsupportedContent = errors.New("unsupported content type")
	ErrFileTooLarge       = errors.New("file exceeds maximum allowed size")
	ErrStorageUnavailable = errors.New("storage backend unavailable")
)

const (
	MaxFileNameLength = 255
	MaxFileSize       = 10 << 20
)

var AllowedContentTypes = map[string]bool{
	"application/pdf":          true,
	"application/json":         true,
	"application/octet-stream": true,
	"application/zip":          true,
	"image/png":                true,
	"image/jpeg":               true,
	"image/gif":                true,
	"image/webp":               true,
	"text/plain":               true,
	"text/csv":                 true,
	"text/html":                true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
}

type StorageMetadata struct {
	Key       string
	Checksum  string
	SizeBytes int64
}

type StorageProvider interface {
	Store(ctx context.Context, key string, reader io.Reader, size int64) (StorageMetadata, error)
	Retrieve(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type StoreOptions struct {
	ContentType string
	FileName    string
	Reader      io.Reader
	Size        int64
}

func SanitizeFileName(name string) (string, error) {
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("%w: filename is empty or dangerous", ErrInvalidFilename)
	}

	cleaned := filepath.Base(name)
	cleaned = strings.ReplaceAll(cleaned, "..", "")
	cleaned = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, cleaned)

	if cleaned == "" {
		return "", fmt.Errorf("%w: filename is empty after sanitization", ErrInvalidFilename)
	}
	if len(cleaned) > MaxFileNameLength {
		return "", fmt.Errorf("%w: filename exceeds %d characters", ErrInvalidFilename, MaxFileNameLength)
	}
	return cleaned, nil
}

func ValidateContentType(ct string) (string, error) {
	if ct == "" {
		return "", fmt.Errorf("%w: content type is required", ErrUnsupportedContent)
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	semicolonIdx := strings.Index(ct, ";")
	if semicolonIdx >= 0 {
		ct = ct[:semicolonIdx]
	}
	ct = strings.TrimSpace(ct)
	if !AllowedContentTypes[ct] {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedContent, ct)
	}
	return ct, nil
}

func ComputeChecksum(reader io.Reader) (string, int64, error) {
	hasher := sha256.New()
	written, err := io.Copy(hasher, reader)
	if err != nil {
		return "", 0, fmt.Errorf("failed to compute checksum: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), written, nil
}

func GenerateStorageKey(evidenceID uuid.UUID, fileName string) string {
	ts := time.Now().UTC().Format("2006/01/02")
	ext := filepath.Ext(fileName)
	return fmt.Sprintf("%s/%s%s", ts, evidenceID.String(), ext)
}
