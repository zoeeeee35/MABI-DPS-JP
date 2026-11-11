//go:build !native

package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTP(t *testing.T) {
	q := fixture(t)
	data, _ := json.Marshal(q)
	request := httptest.NewRequest("POST", "http://127.0.0.1:8039/api/analyze", bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler().ServeHTTP(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "サンプル・アオイ") {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, url := range []string{"http://evil.example:8039/api/analyze", "http://127.0.0.1:8039/api/analyze"} {
		req := httptest.NewRequest("POST", url, bytes.NewReader(data))
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		handler().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
}
