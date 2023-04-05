package main

import (
	// "fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	// "nhooyr.io/websocket"
)

type User struct {
	id   uuid.UUID
	name string
}

type Message struct {
	user *User
	time time.Time
	body string
}

type Server struct {
	mux http.ServeMux

	messageMutex sync.Mutex
	messages     map[*Message]struct{}

	usersMutex sync.Mutex
	users      map[*User]struct{}

	authChannel chan<- AuthCommand
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.mux.ServeHTTP(w, r)
}

func (server *Server) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	authorizationHeader := r.Header.Get("Authorization")
	authorization := strings.Split(authorizationHeader, " ")
	if authorizationHeader == "" || authorization[0] != "Bearer" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Invalid authorization header"))
		return
	}
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: ConsumeToken, reply: replyChannel, token: authorization[1]}
	reply := <-replyChannel
	if reply.authorized {
		w.Write([]byte("200 - authorized"))
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("401 - unauthorized"))
	}
}

func createServer() *Server {
	var server Server
	server.authChannel = startAuthService()
	server.mux.HandleFunc("/join", server.joinRoomHandler)
	return &server
}
