package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/teaelephant/TeaElephantMemory/internal/auth"
)

func TestNewAPNSClientPreservesKeyFileError(t *testing.T) {
	cfg := &auth.Configuration{SecretPath: filepath.Join(t.TempDir(), "missing.p8")}

	client, err := newAPNSClient(cfg, "sandbox")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("newAPNSClient() error = %v, want missing key file error", err)
	}

	if client != nil {
		t.Error("newAPNSClient() returned a client without a readable key")
	}
}
