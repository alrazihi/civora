package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalStorageProvider struct {
	rootDir string
}

func NewLocalStorageProvider(rootDir string) (*LocalStorageProvider, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve storage path: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &LocalStorageProvider{rootDir: absRoot}, nil
}

func (p *LocalStorageProvider) Store(ctx context.Context, key string, reader io.Reader, size int64) (StorageMetadata, error) {
	if size > MaxFileSize {
		return StorageMetadata{}, fmt.Errorf("%w: %d bytes exceeds limit of %d bytes", ErrFileTooLarge, size, MaxFileSize)
	}

	destPath := filepath.Join(p.rootDir, key)

	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return StorageMetadata{}, fmt.Errorf("failed to create directory: %w", err)
	}

	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return StorageMetadata{}, fmt.Errorf("failed to create file: %w", err)
	}
	defer func() { _ = f.Close() }()

	hasher := newChecksumHasher()
	tee := io.TeeReader(reader, hasher)

	written, err := io.Copy(f, tee)
	if err != nil {
		_ = os.Remove(destPath)
		return StorageMetadata{}, fmt.Errorf("failed to write file: %w", err)
	}

	if written != size && size > 0 {
		_ = os.Remove(destPath)
		return StorageMetadata{}, fmt.Errorf("%w: declared size %d does not match actual %d", ErrInvalidFilename, size, written)
	}

	return StorageMetadata{
		Key:       key,
		Checksum:  hasher.Sum(),
		SizeBytes: written,
	}, nil
}

func (p *LocalStorageProvider) Retrieve(ctx context.Context, key string) (io.ReadCloser, error) {
	destPath := filepath.Join(p.rootDir, key)

	cleanPath := filepath.Clean(destPath)
	if !strings.HasPrefix(cleanPath, p.rootDir) {
		return nil, fmt.Errorf("%w: path traversal detected", ErrInvalidFilename)
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: file not found", ErrStorageUnavailable)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, nil
}

func (p *LocalStorageProvider) Delete(ctx context.Context, key string) error {
	destPath := filepath.Join(p.rootDir, key)

	cleanPath := filepath.Clean(destPath)
	if !strings.HasPrefix(cleanPath, p.rootDir) {
		return fmt.Errorf("%w: path traversal detected", ErrInvalidFilename)
	}

	if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}
