package crawler

import (
	"testing"
)

func TestDecryptText(t *testing.T) {
	// Sample directly from Fanqie reader page for "起点"
	encrypted := "“\ue521\ue43f夏临，\ue541\ue53d\ue41e\ue4de\ue485\ue53e\ue4de\ue469\ue50f，\ue521\ue4f3\ue3ec族\ue407\ue490\ue4ed普\ue42b\ue4c2\ue480\ue41c\ue3eb殊。"
	expected := "“我叫夏临，目前是一位高一新生，我的家族相比于普通人有些特殊。"

	decrypted := DecryptText(encrypted)
	if decrypted != expected {
		t.Fatalf("unexpected decrypted text.\nGot:  %s\nWant: %s", decrypted, expected)
	}

	remaining := RemainingPUA(decrypted)
	if len(remaining) != 0 {
		t.Fatalf("expected 0 remaining PUA, got: %v", remaining)
	}
}
