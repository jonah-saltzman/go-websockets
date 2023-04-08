package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type Server struct {
	mux http.ServeMux

	usersMutex sync.Mutex
	users      map[*User]struct{}

	authChannel chan<- AuthCommand
	msgChannel  chan<- MessageCommand
}

type User struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	from chan []byte
	to   chan *[]byte
}

type WsErrorResponse struct {
	Err string `json:"err"`
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.mux.ServeHTTP(w, r)
}

func (server *Server) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: CheckToken, reply: replyChannel, token: token}
	reply := <-replyChannel
	if reply.authorized {
		server.joinRoom(w, r, reply.user)
	} else {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
	}
}

func (server *Server) joinRoom(w http.ResponseWriter, r *http.Request, user *User) error {
	connection, err := websocket.Accept(w, r, nil)
	ctx := r.Context()
	if err != nil {
		return err
	}
	defer connection.Close(websocket.StatusInternalError, "Unknown server error")
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
			replyChan := make(chan MessageCommandResponse)
			server.msgChannel <- MessageCommand{typ: NewMessage, msg: &Message{User: user, Time: time.Now(), Body: string(msg)}, reply: replyChan}
			reply := <-replyChan
			if reply.err != NoMsgError {
				errBytes, _ := json.Marshal(WsErrorResponse{Err: "failed to send message"})
				connection.Write(ctx, websocket.MessageText, errBytes)
			}
		case jsonBytes := <-user.to:
			connection.Write(ctx, websocket.MessageText, *jsonBytes)
		}
	}
	return errors.New("connection closed")
}

func (server *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	var login LoginRequest
	err := json.NewDecoder(r.Body).Decode(&login)
	if err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}
	user := &User{from: make(chan []byte, 10), to: make(chan *[]byte, 10), Name: login.User, Id: uuid.New()}
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: CreateToken, reply: replyChannel, user: user, password: login.Password}
	reply := <-replyChannel
	if reply.err != NoAuthError {
		switch reply.err {
		case InvalidPassword:
			http.Error(w, "Invalid password", http.StatusUnauthorized)
		case TokenGeneration:
			http.Error(w, "Server error", http.StatusInternalServerError)
		}
		return
	}
	w.Write([]byte(reply.token))
}

func (server *Server) getHistoryHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "-1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		http.Error(w, "invalid page parameter", http.StatusBadRequest)
		return
	}
	token, err := parseTokenFromHeader(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user := server.checkToken(token)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	replyChannel := make(chan MessageCommandResponse)
	server.msgChannel <- MessageCommand{typ: GetMessages, page: page, reply: replyChannel}
	reply := <-replyChannel
	if reply.err != NoMsgError {
		switch true {
		case reply.err == JsonError || reply.err == ServerError:
			http.Error(w, "server error", http.StatusInternalServerError)
		case reply.err == BadRequest:
			http.Error(w, "bad request", http.StatusBadRequest)
		}
		return
	}
	w.Write([]byte(*reply.messagesJson))
}

func parseTokenFromHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	slice := strings.Split(authorization, " ")
	if len(slice) < 2 || slice[0] != "Bearer" || slice[1] == "" {
		return "", errors.New("invalid header format")
	}
	return slice[1], nil
}

func (server *Server) checkToken(tokenString string) *User {
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: CheckToken, token: tokenString, reply: replyChannel}
	reply := <-replyChannel
	return reply.user
}

func createServer(password string) (*Server, error) {
	var server Server
	var err error
	server.users = make(map[*User]struct{})
	server.authChannel, err = startAuthService(password)
	if err != nil {
		return nil, errors.New("failed to start the auth service")
	}
	server.msgChannel = startMessageService(&server)
	server.mux.Handle("/", http.FileServer(http.Dir("./client")))
	server.mux.HandleFunc("/join", server.joinRoomHandler)
	server.mux.HandleFunc("/login", server.loginHandler)
	server.mux.HandleFunc("/history", server.getHistoryHandler)
	fmt.Println("started server")
	return &server, nil
}

func (server *Server) addUser(user *User) {
	server.usersMutex.Lock()
	server.users[user] = struct{}{}
	server.usersMutex.Unlock()
}

func (server *Server) removeUser(user *User) {
	server.usersMutex.Lock()
	delete(server.users, user)
	server.usersMutex.Unlock()
}
