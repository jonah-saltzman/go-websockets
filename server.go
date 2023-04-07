package main

import (
	// "fmt"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"
)

type User struct {
	id   uuid.UUID
	name string
	recv chan []byte
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
	err, token, username := validateJoinReq(w, r)
	if err != nil {
		fmt.Println(err)
		return
	}
	replyChannel := make(chan AuthCommandResponse)
	server.authChannel <- AuthCommand{typ: ConsumeToken, reply: replyChannel, token: token}
	reply := <-replyChannel
	if reply.authorized {
		server.joinRoom(w, r, username)
	} else {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("401 - unauthorized"))
	}
}

func validateJoinReq(w http.ResponseWriter, r *http.Request) (error, string, string) {
	token := r.URL.Query().Get("token")
	user := r.URL.Query().Get("user")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Missing token"))
		return errors.New("invalid authorization header"), "", ""
	}
	if user == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Missing user field"))
		return errors.New("missing user field"), "", ""
	}
	return nil, token, user
}

func (server *Server) joinRoom(w http.ResponseWriter, r *http.Request, username string) error {
	fmt.Println("joining room")
	connection, err := websocket.Accept(w, r, nil)
	ctx := r.Context()
	if err != nil {
		return err
	}
	defer connection.Close(websocket.StatusInternalError, "Unknown server error")

	user := &User{recv: make(chan []byte), name: username, id: uuid.New()}
	server.addUser(user)
	defer server.removeUser(user)

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
Outer:
	for {
		select {
		case msg := <-user.recv:
			connection.Write(ctx, websocket.MessageText, msg)
		case msg, ok := <-sent:
			if !ok {
				break Outer
			}
			fmt.Printf("%s\n", msg)
		}
	}
	return errors.New("connection closed")
}

func createServer() *Server {
	var server Server
	server.users = make(map[*User]struct{})
	server.messages = make(map[*Message]struct{})
	server.authChannel = startAuthService()
	server.mux.HandleFunc("/join", server.joinRoomHandler)
	server.mux.Handle("/", http.FileServer(http.Dir("./client")))
	return &server
}

func (server *Server) addUser(user *User) {
	fmt.Printf("add user %s\n", user.name)
	server.usersMutex.Lock()
	server.users[user] = struct{}{}
	server.usersMutex.Unlock()
}

func (server *Server) removeUser(user *User) {
	fmt.Printf("remove user %s\n", user.name)
	server.usersMutex.Lock()
	delete(server.users, user)
	server.usersMutex.Unlock()
}
