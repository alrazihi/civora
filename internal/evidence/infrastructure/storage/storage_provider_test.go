package storage

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple", "document.pdf", "document.pdf", false},
		{"dashes and dots", "my-report_v2.0.pdf", "my-report_v2.0.pdf", false},
		{"traversal stripped", "../../etc/passwd", "passwd", false},
		{"windows path", `C:\Users\test\file.pdf`, "file.pdf", false},
		{"absolute unix path", "/home/user/file.pdf", "file.pdf", false},
		{"empty string", "", "", true},
		{"dots only", "..", "", true},
		{"only dots and slashes", "../../..", "", true},
		{"spaces preserved", "my file.pdf", "my file.pdf", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizeFileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizeFileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SanitizeFileName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"application/pdf", "application/pdf", false},
		{"  APPLICATION/PDF  ", "application/pdf", false},
		{"application/pdf; charset=utf-8", "application/pdf", false},
		{"application/json", "application/json", false},
		{"image/png", "image/png", false},
		{"text/plain", "text/plain", false},
		{"application/x-msdownload", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateContentType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContentType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateContentType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeChecksum(t *testing.T) {
	data := []byte("hello world")
	reader := bytes.NewReader(data)
	checksum, written, err := ComputeChecksum(reader)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	if written != int64(len(data)) {
		t.Errorf("written = %d, want %d", written, len(data))
	}
	if checksum == "" {
		t.Error("checksum is empty")
	}
	if len(checksum) != 64 {
		t.Errorf("checksum length = %d, want 64", len(checksum))
	}
}

func TestGenerateStorageKey(t *testing.T) {
	id := uuid.New()
	key := GenerateStorageKey(id, "report.pdf")
	if !strings.HasPrefix(key, filepath.ToSlash(filepath.Dir(key))+"/") {
		t.Errorf("key = %q, expected date-prefixed path", key)
	}
	if !strings.Contains(key, id.String()) {
		t.Errorf("key = %q, expected evidence ID %s in key", key, id.String())
	}
	if !strings.HasSuffix(key, ".pdf") {
		t.Errorf("key = %q, expected .pdf extension", key)
	}
}

func TestLocalStorageProvider_StoreAndRetrieve(t *testing.T) {
	provider, err := NewLocalStorageProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorageProvider failed: %v", err)
	}

	ctx := context.Background()
	key := "test/" + uuid.New().String() + ".txt"
	content := []byte("test document content")

	md, err := provider.Store(ctx, key, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if md.Key != key {
		t.Errorf("md.Key = %q, want %q", md.Key, key)
	}
	if md.SizeBytes != int64(len(content)) {
		t.Errorf("md.SizeBytes = %d, want %d", md.SizeBytes, len(content))
	}
	if md.Checksum == "" {
		t.Error("md.Checksum is empty")
	}

	rc, err := provider.Retrieve(ctx, key)
	if err != nil {
		t.Fatalf("Retrieve failed: %v", err)
	}
	defer rc.Close()

	buf := make([]byte, len(content))
	_, err = rc.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if string(buf) != string(content) {
		t.Errorf("retrieved content = %q, want %q", string(buf), string(content))
	}
}

func TestLocalStorageProvider_RetrieveNotFound(t *testing.T) {
	provider, err := NewLocalStorageProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorageProvider failed: %v", err)
	}

	ctx := context.Background()
	_, err = provider.Retrieve(ctx, "nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLocalStorageProvider_Delete(t *testing.T) {
	provider, err := NewLocalStorageProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorageProvider failed: %v", err)
	}

	ctx := context.Background()
	key := "test/" + uuid.New().String() + ".txt"
	content := []byte("delete me")

	_, err = provider.Store(ctx, key, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	err = provider.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = provider.Retrieve(ctx, key)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestLocalStorageProvider_StoreTooLarge(t *testing.T) {
	provider, err := NewLocalStorageProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorageProvider failed: %v", err)
	}

	ctx := context.Background()
	key := "test/" + uuid.New().String() + ".txt"
	largeData := bytes.NewReader(make([]byte, MaxFileSize+1))

	_, err = provider.Store(ctx, key, largeData, MaxFileSize+1)
	if err == nil {
		t.Error("expected error for oversized file")
	}
}

func TestLocalStorageProvider_RetrievePathTraversal(t *testing.T) {
	provider, err := NewLocalStorageProvider(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStorageProvider failed: %v", err)
	}

	ctx := context.Background()
	_, err = provider.Retrieve(ctx, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}
