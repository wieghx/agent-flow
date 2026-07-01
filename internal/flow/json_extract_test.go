package flow

import "testing"

func TestExtractJSONObjectFromNoisyOutline(t *testing.T) {
	raw := `some reasoning text
{
  "title": "孤岛法则",
  "synopsis": "简介",
  "characters": [{"name":"林默","role":"主角","trait":"冷静"}],
  "chapters": [
    {"num":1,"title":"第一章","summary":"冲突与转折"},
    {"num":2,"title":"第二章","summary":"高潮与结局"}
  ]
}
more thinking`
	got := ExtractJSONObject(raw, "title", "chapters")
	if got == "" {
		t.Fatal("expected JSON extraction")
	}
	if !stringsContains(got, `"chapters"`) || !stringsContains(got, `"title"`) {
		t.Fatalf("unexpected json: %s", got)
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOfSubstring(s, sub) >= 0)
}

func indexOfSubstring(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
