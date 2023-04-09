package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/jonah-saltzman/go-websockets/auth"
	"github.com/jonah-saltzman/go-websockets/msg"
	"nhooyr.io/websocket"
)

// The server and its methods tie together the http handlers, the auth
// service, and the message service
type Server struct {
	mux http.ServeMux

	usersMutex sync.RWMutex
	users      map[*auth.User]struct{}

	authChannel chan<- auth.AuthCommand
	msgChannel  chan<- msg.MessageCommand
}

func CreateServer(password string) (*Server, error) {
	var server Server
	var err error
	server.users = make(map[*auth.User]struct{})
	server.authChannel, err = auth.StartAuthService(password)
	if err != nil {
		return nil, errors.New("failed to start the auth service")
	}
	server.msgChannel = msg.StartMessageService(&server)
	server.mux.Handle("/", http.FileServer(http.Dir("./client/build")))
	server.mux.HandleFunc("/join", server.joinRoomHandler)
	server.mux.HandleFunc("/login", server.loginHandler)
	server.mux.HandleFunc("/history", server.getHistoryHandler)
	server.mux.HandleFunc("/logout", server.logoutHandler)
	return &server, nil
}

func (server *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	server.mux.ServeHTTP(w, r)
}

// Validates a request to join the chatroom, checking the token with the auth service
// upgrading the request to a websocket connection, and subscribing the user
// to the message service
func (server *Server) joinRoomHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}
	user := server.checkToken(token, false)
	if user == nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	connection, err := websocket.Accept(w, r, nil)
	ctx := r.Context()
	if err != nil {
		http.Error(w, "Websocket error", http.StatusInternalServerError)
		return
	}
	defer connection.Close(websocket.StatusInternalError, "Unknown server error")
	userJson, err := json.Marshal(*user)
	if err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	connection.Write(ctx, websocket.MessageText, userJson)
	msg.SubscribeUser(server, user, ctx, connection.Read, connection.Write)
}

// Validates login requests and passes them to the auth service for authentication
func (server *Server) loginHandler(w http.ResponseWriter, r *http.Request) {
	var login auth.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&login)
	if err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}
	user := &auth.User{Out: make(chan *[]byte, 10), Name: login.User, Id: uuid.New()}
	replyChannel := make(chan auth.AuthCommandResponse)
	server.authChannel <- auth.AuthCommand{Typ: auth.CreateToken, Reply: replyChannel, User: user, Password: login.Password}
	reply := <-replyChannel
	if reply.Err != auth.NoError {
		switch reply.Err {
		case auth.InvalidPassword:
			http.Error(w, "Invalid password", http.StatusUnauthorized)
		case auth.TokenGeneration:
			http.Error(w, "Server error", http.StatusInternalServerError)
		}
		return
	}
	tokenJson, _ := json.Marshal(LoginResponse{Token: reply.Token})
	w.Write(tokenJson)
}

// Validates reqests for message history, checks the token with the auth service,
// and retrieves the message history from the message service.
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
	user := server.checkToken(token, false)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	replyChannel := make(chan msg.MessageCommandResponse)
	server.msgChannel <- msg.MessageCommand{Typ: msg.GetMessages, Page: page, Reply: replyChannel}
	reply := <-replyChannel
	if reply.Err != msg.NoError {
		switch true {
		case reply.Err == msg.JsonError || reply.Err == msg.ServerError:
			http.Error(w, "server error", http.StatusInternalServerError)
		case reply.Err == msg.BadRequest:
			http.Error(w, "bad request", http.StatusBadRequest)
		}
		return
	}
	w.Write([]byte(*reply.MessagesJson))
}

func (server *Server) logoutHandler(w http.ResponseWriter, r *http.Request) {
	token, err := parseTokenFromHeader(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user := server.checkToken(token, true)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	} else {
		w.Write([]byte("OK"))
	}
}

// helper functions implement ServerInterface in the messages package

// provides synchronized access to the users map
func (server *Server) IterUsers(f func(u *auth.User) error) error {
	server.usersMutex.RLock()
	defer server.usersMutex.RUnlock()
	for user := range server.users {
		err := f(user)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseTokenFromHeader(header http.Header) (string, error) {
	authorization := header.Get("Authorization")
	slice := strings.Split(authorization, " ")
	if len(slice) < 2 || slice[0] != "Bearer" || slice[1] == "" {
		return "", errors.New("invalid header format")
	}
	return slice[1], nil
}

func (server *Server) checkToken(tokenString string, consume bool) *auth.User {
	replyChannel := make(chan auth.AuthCommandResponse)
	cmd := auth.AuthCommand{Token: tokenString, Reply: replyChannel}
	if consume {
		cmd.Typ = auth.ConsumeToken
	} else {
		cmd.Typ = auth.CheckToken
	}
	server.authChannel <- cmd
	reply := <-replyChannel
	return reply.User
}

func (server *Server) AddUser(user *auth.User) {
	server.usersMutex.Lock()
	server.users[user] = struct{}{}
	server.usersMutex.Unlock()
}

func (server *Server) RemoveUser(user *auth.User) {
	server.usersMutex.Lock()
	delete(server.users, user)
	server.usersMutex.Unlock()
}

func (server *Server) SendMsgCmd(cmd msg.MessageCommand) {
	server.msgChannel <- cmd
}

type LoginResponse struct {
	Token string `json:"token"`
}
