package widget

import (
	"os"
	"strings"
	"testing"
)

// TestGenerateMatchesNodeGolden 与 Node 原版 lib/widget.js 生成的输出逐行对比
func TestGenerateMatchesNodeGolden(t *testing.T) {
	golden, err := os.ReadFile("testdata/widget_golden.txt")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := Generate("http://vps.example.com:5555")
	if got == string(golden) {
		return
	}
	gl := strings.Split(string(golden), "\n")
	tl := strings.Split(got, "\n")
	for i := 0; i < len(gl) || i < len(tl); i++ {
		var g, l string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(tl) {
			l = tl[i]
		}
		if g != l {
			t.Fatalf("first diff at line %d:\n  golden: %q\n  got:    %q", i+1, g, l)
		}
	}
}

func TestGenerateBaseURL(t *testing.T) {
	if !strings.Contains(Generate("http://x.com:1/"), `const BASE = "http://x.com:1";`) {
		t.Error("trailing slash not trimmed")
	}
	if !strings.Contains(Generate(""), `const BASE = "http://127.0.0.1:5555";`) {
		t.Error("empty base url fallback failed")
	}
}
