package crawler

import (
	"context"
	"testing"
	"time"
)

func TestGetLoginQRCodeReal(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ticket, err := client.GetLoginQRCode(ctx)
	if err != nil {
		t.Fatalf("GetLoginQRCode failed: %v", err)
	}

	if ticket.Token == "" {
		t.Fatalf("expected non-empty token")
	}
	if ticket.QRCodeURL == "" {
		t.Fatalf("expected non-empty qrcode URL")
	}
	if len(ticket.QRArt) == 0 {
		t.Fatalf("expected non-empty QR art")
	}

	t.Logf("Got QR ticket: Token=%s, URL=%s, Art length=%d",
		ticket.Token, ticket.QRCodeURL, len(ticket.QRArt))

	// Test check_qrconnect status immediately (should be "new")
	status, _, err := client.CheckQRConnect(ctx, ticket.Token)
	if err != nil {
		t.Fatalf("CheckQRConnect failed: %v", err)
	}
	if status != "new" {
		t.Logf("check status: %s", status)
	}
}
