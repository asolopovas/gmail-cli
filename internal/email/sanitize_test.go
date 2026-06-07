package email

import "testing"

func TestSafeFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "path traversal basename", in: "../../secret.txt", want: "secret.txt"},
		{name: "unsafe characters", in: "invoice: Q1/2026?.pdf", want: "2026_.pdf"},
		{name: "blank fallback", in: "   ", want: "fallback"},
		{name: "dots fallback", in: "..", want: "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SafeFilename(tt.in, "fallback", 80)
			if got != tt.want {
				t.Fatalf("SafeFilename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSafeLabelPreservesDescriptiveTextWithoutPathTraversal(t *testing.T) {
	t.Parallel()
	got := SafeLabel("26-06-07 142233 - ../../Quarterly/Report?", "fallback", 120)
	want := "26-06-07 142233 - .._.._Quarterly_Report_"
	if got != want {
		t.Fatalf("SafeLabel() = %q, want %q", got, want)
	}
}
