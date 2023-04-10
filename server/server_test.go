package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jonah-saltzman/go-websockets/auth"
)

var s *Server

var goodLogin, _ = json.Marshal(auth.LoginRequest{
	User:     "jonah",
	Password: "look24",
})

func TestMain(m *testing.M) {
	server, err := CreateServer("look24")
	if err != nil {
		os.Exit(1)
	}
	s = server
	code := m.Run()
	os.Exit(code)
}

func TestLoginHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(goodLogin))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler := http.HandlerFunc(s.loginHandler)
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("non-200 status code")
	}
	token := w.Body.String()
	if len(token) <= 0 {
		t.Errorf("token '%s' invalid", token)
	}
}
