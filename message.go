package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	User *User     `json:"user"`
	Time time.Time `json:"time"`
	Body string    `json:"body"`
}

type User struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	from chan []byte
	to   chan *[]byte
}

func startMessageService(server *Server) chan<- *Message {
	messages := make(chan *Message, 100)
	go func() {
		for msg := range messages {
			msgBytes, err := json.Marshal(msg)
			if err != nil {
				fmt.Println(err)
				continue
			}
			server.usersMutex.Lock()
			for u := range server.users {
				u.to <- &msgBytes
			}
			server.usersMutex.Unlock()
			server.messageMutex.Lock()
			server.messages[msg] = struct{}{}
			server.messageMutex.Unlock()
		}
	}()
	return messages
}
