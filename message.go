package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"time"
)

type MessageCommandType int

const (
	NewMessage = iota
	GetMessages
)

type MessageCommandError int

const (
	NoMsgError = iota
	JsonError
	BadRequest
	ServerError
)

type MessageCommand struct {
	typ   MessageCommandType
	msg   *Message
	page  int
	reply chan MessageCommandResponse
}

type MessageCommandResponse struct {
	messagesJson *string
	nextPage     int
	err          MessageCommandError
}

type Message struct {
	User *User     `json:"user"`
	Time time.Time `json:"time"`
	Body string    `json:"body"`
}

type MessageBucket struct {
	Id        int        `json:"page"`
	Messages  []*Message `json:"messages,omitempty"`
	json      *string
	jsonStale bool
}

// the message service is a goroutine which handles incoming
// messages and paged requests for message history
func startMessageService(server *Server) chan<- MessageCommand {
	msgCommands := make(chan MessageCommand, 100)
	buckets := list.New()
	buckets.PushBack(&MessageBucket{Id: 0, Messages: make([]*Message, 0, 100), json: nil, jsonStale: true})
	newMessageHandler := server.getNewMessageHandler(buckets)
	getMessagesHandler := getGetMessagesHandler(buckets)
	go func() {
		for cmd := range msgCommands {
			switch cmd.typ {
			case NewMessage:
				newMessageHandler(cmd)
			case GetMessages:
				getMessagesHandler(cmd)
			}
		}
	}()
	return msgCommands
}

// after a bucket is full, it will be serialized a maximum of one time, after
// which all subsequent requests for the same bucket will return the same *string
func getGetMessagesHandler(buckets *list.List) func(MessageCommand) {
	return func(mc MessageCommand) {
		e := buckets.Back()
		if mc.page > e.Value.(*MessageBucket).Id || mc.page < -1 {
			mc.reply <- MessageCommandResponse{err: BadRequest}
			return
		}
		if mc.page != -1 {
			for ; e.Value.(*MessageBucket).Id > mc.page && e != nil; e = e.Prev() {
			}
		}
		if e == nil {
			mc.reply <- MessageCommandResponse{err: ServerError}
			return
		}
		bucket := e.Value.(*MessageBucket)
		if bucket.jsonStale {
			json, err := json.Marshal(*bucket)
			if err != nil {
				mc.reply <- MessageCommandResponse{err: JsonError}
			}
			str := string(json)
			bucket.json = &str
			bucket.jsonStale = false
		}
		// we can pass a pointer to the json string through the channel because this string will
		// never be modified. If a new message comes in, a new json string will be generated
		mc.reply <- MessageCommandResponse{messagesJson: bucket.json, nextPage: bucket.Id - 1}
	}
}

// broadcasts the msg and puts it in a bucket, creating a new bucket if current one
// is full
func (server *Server) getNewMessageHandler(buckets *list.List) func(MessageCommand) {
	currId := 1
	return func(mc MessageCommand) {
		msgBytes, err := json.Marshal(mc.msg)
		if err != nil {
			fmt.Println(err)
			mc.reply <- MessageCommandResponse{err: JsonError}
			return
		}
		server.usersMutex.Lock()
		for u := range server.users {
			u.to <- &msgBytes
		}
		server.usersMutex.Unlock()
		currBucket := buckets.Back().Value.(*MessageBucket)
		if len(currBucket.Messages) < 100 {
			currBucket.Messages = append(currBucket.Messages, mc.msg)
			currBucket.jsonStale = true
		} else {
			msgSlice := make([]*Message, 0, 100)
			msgSlice[0] = mc.msg
			buckets.PushBack(&MessageBucket{Id: currId, Messages: msgSlice, json: nil, jsonStale: true})
			currId += 1
		}
		mc.reply <- MessageCommandResponse{}
	}
}
