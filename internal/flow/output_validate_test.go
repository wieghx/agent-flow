package flow

import (
	"strconv"
	"testing"
)

func TestValidateWorkerOutputVolumeOutline(t *testing.T) {
	instruction := `为第4卷（第76-100章）生成详细章节大纲
输出 JSON：{"volume":4,"chapters":[{"num":76,"title":"t","summary":"s"}]}
要求：chapters 恰好覆盖第76到第100章`
	complete := `{"volume":4,"chapters":[`
	for n := 76; n <= 100; n++ {
		if n > 76 {
			complete += ","
		}
		complete += `{"num":` + strconv.Itoa(n) + `,"title":"章","summary":"梗概"}`
	}
	complete += `]}`
	if err := ValidateWorkerOutput(instruction, complete, TaskTypeNovelVolumeOutline); err != nil {
		t.Fatalf("complete volume outline should pass: %v", err)
	}
	truncated := `{"volume":4,"chapters":[{"num":76,"title":"章","summary":"梗概"}]}`
	if err := ValidateWorkerOutput(instruction, truncated, TaskTypeNovelVolumeOutline); err == nil {
		t.Fatal("truncated volume outline should fail")
	}
}
