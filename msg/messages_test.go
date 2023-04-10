package msg_test

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonah-saltzman/go-websockets/auth"
	"github.com/jonah-saltzman/go-websockets/msg"
	"github.com/jonah-saltzman/go-websockets/server"
	"nhooyr.io/websocket"
)

func randomString(b int) string {
	bytes := make([]byte, b)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// 1000 users join the room and send 1 message each.
// This test verifies that all 1000 users receive one
// copy of all 1000 messages.
func TestBroadcast(t *testing.T) {
	s, _ := server.CreateServer("look24")
	var joinWg sync.WaitGroup
	var sentWg sync.WaitGroup
	var doneWg sync.WaitGroup
	clients := 1000
	joinWg.Add(clients)
	sentWg.Add(clients)
	doneWg.Add(clients)
	for i := 0; i < clients; i++ {
		go func(myI int) {
			defer doneWg.Done()
			randomUser := auth.User{Id: uuid.New(), Name: randomString(5), Out: make(chan *[]byte)}
			received := make(map[int]struct{})
			mockWrite := func(ctx context.Context, typ websocket.MessageType, p []byte) error {
				var msg msg.Message
				err := json.Unmarshal(p, &msg)
				if err != nil {
					t.Errorf("bad json: %s\n", err.Error())
				}
				n, err := strconv.Atoi(msg.Body)
				if err != nil {
					t.Errorf("bad body: %s\n", err.Error())
				}
				received[n] = struct{}{}
				return nil
			}
			haveRead := false
			mockRead := func(ctx context.Context) (websocket.MessageType, []byte, error) {
				if !haveRead {
					defer sentWg.Done()
					joinWg.Wait()
					haveRead = true
					bytes := []byte(strconv.Itoa(myI))
					return websocket.MessageText, bytes, nil
				}
				select {}
			}
			go msg.SubscribeUser(s, &randomUser, context.Background(), mockRead, mockWrite)
			time.Sleep(time.Millisecond * 10)
			joinWg.Done()
			sentWg.Wait()
			time.Sleep(time.Second)
			if len(received) != clients {
				t.Errorf("incorrect number of messages received: %d\n", len(received))
				return
			}
			for i := 0; i < clients; i++ {
				_, ok := received[i]
				if !ok {
					t.Errorf("[%d]: number missing from map: %d\n", myI, i)
				}
			}
			t.Logf("[%s]: routine successful\n", randomUser.Name)
		}(i)
	}
	doneWg.Wait()
}
