package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
)

type checksumHasher struct {
	h hash.Hash
}

func newChecksumHasher() *checksumHasher {
	return &checksumHasher{h: sha256.New()}
}

func (c *checksumHasher) Write(p []byte) (int, error) {
	return c.h.Write(p)
}

func (c *checksumHasher) Sum() string {
	return hex.EncodeToString(c.h.Sum(nil))
}

var _ io.Writer = (*checksumHasher)(nil)
