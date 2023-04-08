package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Server struct {
	mux http.ServeMux

	messageMutex sync.Mutex
	messages     map[*Message]struct{}

	usersMutex sync.Mutex
	users      map[*User]struct{}

	authChannel chan<- AuthCommand
	msgChannel  chan<- *Message
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.mux.ServeHTTP(w, r)
}

func (server *Server) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
	}
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: ConsumeToken, reply: replyChannel, token: token}
	reply := <-replyChannel
	if reply.authorized {
		server.joinRoom(w, r, reply.user)
	} else {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
	}
}

func (server *Server) joinRoom(w http.ResponseWriter, r *http.Request, username string) error {
	connection, err := websocket.Accept(w, r, nil)
	ctx := r.Context()
	if err != nil {
		return err
	}
	defer connection.Close(websocket.StatusInternalError, "Unknown server error")

	user := &User{from: make(chan []byte), to: make(chan *[]byte), Name: username, Id: uuid.New()}
	server.addUser(user)
	defer server.removeUser(user)

	userJson, err := json.Marshal(*user)
	if err != nil {
		return err
	}

	connection.Write(ctx, websocket.MessageText, userJson)

	sent := make(chan []byte)
	go func() {
		for {
			_, bytes, err := connection.Read(ctx)
			if err != nil {
				fmt.Println(err)
				close(sent)
				return
			}
			sent <- bytes
		}
	}()
outer:
	for {
		select {
		case msg, ok := <-sent:
			if !ok {
				break outer
			}
			server.msgChannel <- &Message{User: user, Time: time.Now(), Body: string(msg)}
		case jsonBytes := <-user.to:
			if err != nil {
				fmt.Println(err)
				continue
			}
			connection.Write(ctx, websocket.MessageText, *jsonBytes)
		}
	}
	return errors.New("connection closed")
}

func (server *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	//fmt.Println("login attempt")
	// w.Header().Set("Access-Control-Allow-Origin", "*")
	var login LoginRequest
	err := json.NewDecoder(r.Body).Decode(&login)
	if err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
	}

	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: CheckPw, reply: replyChannel, user: login.User, password: login.Password}
	reply := <-replyChannel
	if reply.err != NoError {
		switch reply.err {
		case InvalidPassword:
			http.Error(w, "Invalid password", http.StatusUnauthorized)
		case TokenGeneration:
			http.Error(w, "Server error", http.StatusInternalServerError)
		}
	}
	w.Write([]byte(reply.token))
}

func createServer(password string) (*Server, error) {
	var server Server
	var err error
	server.users = make(map[*User]struct{})
	server.messages = make(map[*Message]struct{})
	server.authChannel, err = startAuthService(password)
	if err != nil {
		return nil, errors.New("failed to start the auth service")
	}
	server.msgChannel = startMessageService(&server)
	server.mux.Handle("/", http.FileServer(http.Dir("./client")))
	server.mux.HandleFunc("/join", server.joinRoomHandler)
	server.mux.HandleFunc("/login", server.loginHandler)
	fmt.Println("started server")
	return &server, nil
}

func (server *Server) addUser(user *User) {
	fmt.Printf("user %s joined\n", user.Name)
	server.usersMutex.Lock()
	server.users[user] = struct{}{}
	server.usersMutex.Unlock()
}

func (server *Server) removeUser(user *User) {
	fmt.Printf("removed user %s\n", user.Name)
	server.usersMutex.Lock()
	delete(server.users, user)
	server.usersMutex.Unlock()
}
