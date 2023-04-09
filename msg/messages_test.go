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
	var joinwg sync.WaitGroup
	var sentwg sync.WaitGroup
	var donewg sync.WaitGroup
	clients := 1000
	joinwg.Add(clients)
	sentwg.Add(clients)
	donewg.Add(clients)
	for i := 0; i < clients; i++ {
		go func(myI int) {
			randomUser := auth.User{Id: uuid.New(), Name: randomString(5), Out: make(chan *[]byte)}
			haveRead := false
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
			mockRead := func(ctx context.Context) (websocket.MessageType, []byte, error) {
				if !haveRead {
					joinwg.Wait()
					haveRead = true
					sentwg.Done()
					bytes := []byte(strconv.Itoa(myI))
					return websocket.MessageText, bytes, nil
				}
				select {}
			}
			go msg.SubscribeUser(s, &randomUser, context.Background(), mockRead, mockWrite)
			time.Sleep(time.Millisecond * 10)
			joinwg.Done()
			sentwg.Wait()
			time.Sleep(time.Millisecond * 1000)
			if len(received) != clients {
				t.Errorf("incorrect number of messages received: %d\n", len(received))
				donewg.Done()
				return
			}
			for i := 0; i < clients; i++ {
				_, ok := received[i]
				if !ok {
					t.Errorf("[%d]: number missing from map: %d\n", myI, i)
				}
			}
			t.Logf("[%s]: routine successful\n", randomUser.Name)
			donewg.Done()
		}(i)
	}
	donewg.Wait()
}
