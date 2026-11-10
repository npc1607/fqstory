package masquerade

import (
	"strings"
	"testing"
	"time"
)

func TestFormatLog(t *testing.T) {
	paragraphs := []string{"第一段主角挥剑。", "第二段天地变色。"}
	out := FormatLog(paragraphs, time.Now())
	if !strings.Contains(out, "第一段主角挥剑。") {
		t.Errorf("expected paragraph to be present in log format")
	}
	if !strings.Contains(out, "[worker-pool-") {
		t.Errorf("expected worker pool prefix in log format")
	}
}

func TestFormatCode(t *testing.T) {
	paragraphs := []string{"主角破开大阵。"}
	out := FormatCode(paragraphs, "order")
	if !strings.Contains(out, "package order") {
		t.Errorf("expected package declaration")
	}
	if !strings.Contains(out, "// 主角破开大阵。") {
		t.Errorf("expected comment with paragraph")
	}
}

func TestModeCycle(t *testing.T) {
	m := ModeClassic
	m = m.Next()
	if m != ModeLog {
		t.Errorf("expected ModeLog, got %v", m)
	}
	m = m.Next()
	if m != ModeCode {
		t.Errorf("expected ModeCode, got %v", m)
	}
	m = m.Next()
	if m != ModeStatusLine {
		t.Errorf("expected ModeStatusLine, got %v", m)
	}
	m = m.Next()
	if m != ModeClassic {
		t.Errorf("expected loop back to ModeClassic, got %v", m)
	}

	// Test ExtractMiniTicker
	paras := []string{"第一段主角挥剑。", "第二段天地变色。"}
	visible, pct := ExtractMiniTicker(paras, 0, 8)
	if visible != "第一段主角挥剑。" {
		t.Errorf("expected '第一段主角挥剑。', got %q", visible)
	}
	if pct <= 0 {
		t.Errorf("expected valid percentage, got %d", pct)
	}

	// Test fake screens non-empty
	if len(GenerateFakeGitScreen()) == 0 || len(GenerateFakeDockerScreen()) == 0 {
		t.Errorf("expected fake screens to be non-empty")
	}
}
