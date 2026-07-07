package safepath

import "testing"

func TestResolveUnderRoot(t *testing.T) {
	got, err := ResolveUnderRoot("/data/outputs", "workflows/default/novel/book.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/data/outputs/workflows/default/novel/book.md" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveUnderRootRejectsTraversal(t *testing.T) {
	_, err := ResolveUnderRoot("/data/outputs", "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for traversal")
	}
}