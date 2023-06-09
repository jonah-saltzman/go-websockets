package msg

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jonah-saltzman/go-websockets/auth"
	"nhooyr.io/websocket"
)

const MSG_CHAN_DEPTH int = 10
const MSG_BUCKET_SIZE int = 100

type ServerInterface interface {
	IterUsers(func(u *auth.User) error) error
	AddUser(*auth.User)
	RemoveUser(*auth.User)
	SendMsgCommand(MessageCommand, bool) MessageCommandResponse
}

type MessageCommandType int

const (
	NewMessage = iota
	GetMessages
)

type MessageCommandError int

const (
	NoError = iota
	JsonError
	BadRequest
	ServerError
	UnknownCommand
)

type MessageCommand struct {
	Typ   MessageCommandType
	Msg   *Message
	Page  int
	Reply chan MessageCommandResponse
}

type MessageCommandResponse struct {
	MessagesJson *string
	NextPage     int
	Err          MessageCommandError
}

type Message struct {
	User *auth.User `json:"user"`
	Time time.Time  `json:"time"`
	Body string     `json:"body"`
}

type MessageBucket struct {
	Id        int        `json:"page"`
	Messages  []*Message `json:"messages,omitempty"`
	json      *string
	jsonStale bool
}

type WsErrorResponse struct {
	Err string `json:"err"`
}

// the message service is a goroutine which handles incoming
// messages and paged requests for message history
func StartMessageService(server ServerInterface) chan<- MessageCommand {
	msgCommands := make(chan MessageCommand, MSG_CHAN_DEPTH)
	buckets := list.New()
	buckets.PushBack(&MessageBucket{Id: 0, Messages: make([]*Message, 0, MSG_BUCKET_SIZE), json: nil, jsonStale: true})
	newMessageHandler := getNewMessageHandler(server, buckets)
	getMessagesHandler := getGetMessagesHandler(buckets)
	go func() {
		for cmd := range msgCommands {
			switch cmd.Typ {
			case NewMessage:
				newMessageHandler(cmd)
			case GetMessages:
				getMessagesHandler(cmd)
			default:
				cmd.Reply <- MessageCommandResponse{Err: UnknownCommand}
			}
		}
	}()
	return msgCommands
}

// broadcasts the msg and puts it in a bucket, creating a new bucket if current one
// is full
func getNewMessageHandler(server ServerInterface, buckets *list.List) func(MessageCommand) {
	currId := 1
	return func(cmd MessageCommand) {
		msgBytes, err := json.Marshal(cmd.Msg)
		if err != nil {
			fmt.Println(err)
			return
		}
		server.IterUsers(func(u *auth.User) error {
			u.Out <- &msgBytes
			return nil
		})
		currBucket := buckets.Back().Value.(*MessageBucket)
		if len(currBucket.Messages) < 100 {
			currBucket.Messages = append(currBucket.Messages, cmd.Msg)
			currBucket.jsonStale = true
		} else {
			msgSlice := make([]*Message, 0, MSG_BUCKET_SIZE)
			msgSlice = append(msgSlice, cmd.Msg)
			buckets.PushBack(&MessageBucket{Id: currId, Messages: msgSlice, json: nil, jsonStale: true})
			currId += 1
		}
	}
}

// after a bucket is full, it will be serialized a maximum of one time, after
// which all subsequent requests for the same bucket will return the same *string
func getGetMessagesHandler(buckets *list.List) func(MessageCommand) {
	return func(cmd MessageCommand) {
		e := buckets.Back()
		if cmd.Page > e.Value.(*MessageBucket).Id || cmd.Page < -1 {
			cmd.Reply <- MessageCommandResponse{Err: BadRequest}
			return
		}
		// in most cases a user will only want the 100 most recent messages, so normally
		// we avoid traversing the list. If we do traverse, the traversal should terminate
		// quickly
		if cmd.Page != -1 {
			for ; e.Value.(*MessageBucket).Id > cmd.Page && e != nil; e = e.Prev() {
			}
		}
		if e == nil {
			cmd.Reply <- MessageCommandResponse{Err: ServerError}
			return
		}
		bucket := e.Value.(*MessageBucket)
		// don't reserialize the messages unless necessary
		if bucket.jsonStale {
			json, err := json.Marshal(*bucket)
			if err != nil {
				cmd.Reply <- MessageCommandResponse{Err: JsonError}
			}
			str := string(json)
			bucket.json = &str
			bucket.jsonStale = false
		}
		// we can pass a pointer to the json string through the channel because this string will
		// never be modified. If a new message comes in, a new json string will be generated
		cmd.Reply <- MessageCommandResponse{MessagesJson: bucket.json, NextPage: bucket.Id - 1}
	}
}

// starts two goroutines that form the interface between the message service and the websocket
// connection. Messages from the user are sent to the message service, and the message service
// sends broadcasts to the user.
// These are separate goroutines because I encountered deadlocks with high message volume when
// a single goroutine was responsible for incoming and outgoing messages.
func SubscribeUser(
	server ServerInterface,
	user *auth.User,
	ctx context.Context,
	read func(ctx context.Context) (websocket.MessageType, []byte, error),
	write func(ctx context.Context, typ websocket.MessageType, p []byte) error,
) {
	server.AddUser(user)
	defer server.RemoveUser(user)
	errChan := make(chan struct{})
	doneChan := make(chan struct{})
	go func() {
		for {
			_, bytes, err := read(ctx)
			if err != nil {
				fmt.Println(err)
				close(errChan)
				return
			}
			cmd := MessageCommand{Typ: NewMessage, Msg: &Message{User: user, Time: time.Now(), Body: string(bytes)}}
			server.SendMsgCommand(cmd, false)
		}
	}()
	go func() {
		for {
			select {
			case m := <-user.Out:
				write(ctx, websocket.MessageText, *m)
			case <-doneChan:
				return
			}
		}
	}()
	<-errChan
	close(doneChan)
}
