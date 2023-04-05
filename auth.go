package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/deckarep/golang-set"
)

type AuthCommandType int

const (
	CreateToken = iota
	ConsumeToken
)

type AuthCommandResponse struct {
	authorized bool
	token      string
	e          error
}

type AuthCommand struct {
	typ   AuthCommandType
	token string
	reply chan AuthCommandResponse
}

func startAuthService() chan<- AuthCommand {
	// set doesn't need to be synchronized, only the below goroutine will access it
	tokens := mapset.NewThreadUnsafeSet()

	commands := make(chan AuthCommand)

	go func() {
		for cmd := range commands {
			switch cmd.typ {
			case CreateToken:
				bytes := make([]byte, 32)
				_, err := rand.Read(bytes)
				if err != nil {
					cmd.reply <- AuthCommandResponse{e: errors.New("failed to generate random bytes")}
				}
				token := fmt.Sprintf("%x", bytes)
				tokens.Add(token)
				cmd.reply <- AuthCommandResponse{token: token}
			case ConsumeToken:
				if tokens.Contains(cmd.token) {
					tokens.Remove(cmd.token)
					cmd.reply <- AuthCommandResponse{authorized: true}
				} else {
					cmd.reply <- AuthCommandResponse{authorized: false}
				}
			default:
				cmd.reply <- AuthCommandResponse{e: errors.New("unknown command")}
			}
		}
	}()

	return commands
}
