package crawler

import (
	"testing"
)

func TestExtractBookIDFromText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"7307273223832355390", "7307273223832355390"},
		{"https://fanqienovel.com/page/7307273223832355390", "7307273223832355390"},
		{"https://fanqienovel.com/reader/7307267487517655614", "7307267487517655614"},
		{"快来看《惊鸿》https://fanqienovel.com/page/6883748331202284558 超级好看！", "6883748331202284558"},
		{"https://example.com/api?book_id=7307273223832355390&other=1", "7307273223832355390"},
		{"invalid text with no id", ""},
	}

	for _, tt := range tests {
		got := ExtractBookIDFromText(tt.input)
		if got != tt.expected {
			t.Errorf("ExtractBookIDFromText(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestResolveBookID(t *testing.T) {
	// Pure ID
	id, err := ResolveBookID("6883748331202284558")
	if err != nil || id != "6883748331202284558" {
		t.Fatalf("ResolveBookID bare ID failed: %v, got %s", err, id)
	}

	// Web URL
	id, err = ResolveBookID("https://fanqienovel.com/page/7307273223832355390")
	if err != nil || id != "7307273223832355390" {
		t.Fatalf("ResolveBookID web URL failed: %v, got %s", err, id)
	}
}
