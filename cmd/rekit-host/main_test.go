package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shuiyu486/re-context-kits/internal/rekit/sessionhost"
)

func TestPublishLiveAcceptanceReceiptFailureClearsPassed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	if err := os.WriteFile(path, []byte("existing evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := publishLiveAcceptanceReceipt(path, sessionhost.LiveAcceptanceReceipt{Passed: true}, nil)
	if err == nil || result.Passed || result.ReceiptPublication != "failed" || !strings.Contains(result.ReceiptError, "already exists") {
		t.Fatalf("publication failure result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "existing evidence\n" {
		t.Fatalf("existing receipt changed: %q err=%v", data, readErr)
	}
}

func TestPublishLiveAcceptanceReceiptPersistsPublishedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	result, err := publishLiveAcceptanceReceipt(path, sessionhost.LiveAcceptanceReceipt{Passed: true}, nil)
	if err != nil || !result.Passed || result.ReceiptPublication != "published" || result.ReceiptError != "" {
		t.Fatalf("publication success result=%+v err=%v", result, err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || !strings.Contains(string(data), `"receiptPublication": "published"`) {
		t.Fatalf("durable receipt omitted publication state: %q err=%v", data, readErr)
	}
}
