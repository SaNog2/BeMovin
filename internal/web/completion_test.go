package web

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestViewerCanWrite(t *testing.T) {
	tests := []struct {
		method, path string
		want         bool
	}{
		{"POST", "/trainings/12/completion", true},
		{"DELETE", "/trainings/12/completion", false},
		{"POST", "/trainings/new", false},
		{"POST", "/api/trainings", false},
		{"POST", "/trainings/x/completion", false},
		{"POST", "/trainings/12/completion/x", false},
	}
	for _, tt := range tests {
		if got := ViewerCanWrite(httptest.NewRequest(tt.method, tt.path, nil)); got != tt.want {
			t.Errorf("%s %s: got %v, want %v", tt.method, tt.path, got, tt.want)
		}
	}
}

func TestParseCompletion(t *testing.T) {
	p, msg := parseCompletion(3, "4", "  felt strong \n")
	if msg != "" || !p.Feeling.Valid || p.Feeling.Int16 != 4 || p.CompletionNote != "felt strong" || p.ID != 3 {
		t.Fatalf("got %+v, %q", p, msg)
	}
	if p, msg := parseCompletion(3, "", ""); msg != "" || p.Feeling.Valid {
		t.Fatalf("empty feeling: got %+v, %q", p, msg)
	}
	for _, bad := range []string{"0", "6", "x", "-1"} {
		if _, msg := parseCompletion(3, bad, ""); msg == "" {
			t.Errorf("feeling %q: expected an error", bad)
		}
	}
	if _, msg := parseCompletion(3, "", strings.Repeat("é", maxCompletionNote+1)); msg == "" {
		t.Error("long note: expected an error")
	}
}

func TestViewerCanWriteComments(t *testing.T) {
	for path, want := range map[string]bool{
		"/trainings/12/comments":          true,
		"/trainings/12/comments/3/delete": true,
		"/trainings/12/comments/x/delete": false,
		"/trainings/12/comments/3":        false,
	} {
		if got := ViewerCanWrite(httptest.NewRequest("POST", path, nil)); got != want {
			t.Errorf("POST %s: got %v, want %v", path, got, want)
		}
	}
}

func TestParseComment(t *testing.T) {
	if b, msg := parseComment("  nice one \n"); b != "nice one" || msg != "" {
		t.Fatalf("got %q, %q", b, msg)
	}
	if _, msg := parseComment(" \n "); msg == "" {
		t.Error("blank: expected an error")
	}
	if _, msg := parseComment(strings.Repeat("a", maxCommentLen+1)); msg == "" {
		t.Error("long: expected an error")
	}
}
