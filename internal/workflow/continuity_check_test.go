package workflow

import "testing"

func TestContinuityWeak(t *testing.T) {
	prev := "苏晴收起录音笔，转身离开了废弃工厂的大门。"
	open := "太空中，飞船缓缓驶离银河系边缘。"
	if !ContinuityWeak(prev, open) {
		t.Fatal("expected weak continuity")
	}
	open2 := "苏晴走出大门，夜雨仍未停。"
	if ContinuityWeak(prev, open2) {
		t.Fatal("expected strong continuity")
	}
}

func TestContinuityWeak_commonBigramIgnored(t *testing.T) {
	prev := "他缓缓转过身，消失在了夜色之中。"
	open := "另一处，她缓缓抬起头望向星空。"
	if !ContinuityWeak(prev, open) {
		t.Fatal("shared common bigram 缓缓 should not imply continuity")
	}
}