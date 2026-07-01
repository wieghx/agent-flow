package workflow

import (
	"strings"
	"testing"
)

func TestBuildConsistencyAnchor(t *testing.T) {
	base := `书名: 质检测试短篇
主要人物（必须保持一致）:
- 苏晴（记者）: 冷静敏锐
当前章节: 第1章《雨夜》
本章梗概: 苏晴潜入废弃工厂取证。`
	anchor := BuildConsistencyAnchor(base)
	if !strings.Contains(anchor, "苏晴") {
		t.Fatalf("missing protagonist: %s", anchor)
	}
	if !strings.Contains(anchor, "雨夜") {
		t.Fatalf("missing chapter: %s", anchor)
	}
	if !strings.Contains(anchor, "一致性锚点") {
		t.Fatal("missing anchor header")
	}
}