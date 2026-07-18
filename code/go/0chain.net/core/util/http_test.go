package util

import (
	"testing"
)

func TestNewHTTPRequestReturnsErrorForInvalidMethod(t *testing.T) {
	// An invalid method makes http.NewRequest return a nil request and an
	// error. NewHTTPRequest must surface that error instead of dereferencing
	// the nil request.
	req, _, cncl, err := NewHTTPRequest("bad method", "http://example.com", nil)
	if cncl != nil {
		defer cncl()
	}
	if err == nil {
		t.Fatal("expected an error for an invalid method, got nil")
	}
	if req != nil {
		t.Fatalf("expected nil request on error, got %v", req)
	}
}
