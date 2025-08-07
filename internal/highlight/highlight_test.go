package highlight

import "testing"

func TestColorizeLine(t *testing.T) {
	if got := ColorizeLine("ERROR something bad"); got[:5] != "[red]" {
		t.Fatalf("expected red tag, got %q", got)
	}
	if got := ColorizeLine("WARN something"); got[:8] != "[orange]" {
		t.Fatalf("expected orange tag, got %q", got)
	}
	if got := ColorizeLine("just text"); got != "just text" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}
