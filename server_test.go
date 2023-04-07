package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

var s *Server

var goodLogin, _ = json.Marshal(LoginRequest{
	User:     "jonah",
	Password: "look24",
})

func TestMain(m *testing.M) {
	server, err := createServer("look24")
	if err != nil {
		os.Exit(1)
	}
	s = server
	code := m.Run()
	os.Exit(code)
}

// func TestLoginHandler(t *testing.T) {
// 	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(goodLogin))
// 	req.Header.Set("Content-Type", "application/json")
// 	w := httptest.NewRecorder()
// 	handler := http.HandlerFunc(s.loginHandler)
// 	handler.ServeHTTP(w, req)
// 	if w.Code != 200 {
// 		t.Errorf("non-200 status code")
// 	}
// 	token := w.Body.String()
// 	if len(token) <= 0 {
// 		t.Errorf("token '%s' invalid", token)
// 	}
// }

func BenchmarkLogin(b *testing.B) {
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(goodLogin))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := http.HandlerFunc(s.loginHandler)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler(w, req)
	}
}