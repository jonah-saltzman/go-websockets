package main

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type AuthCommandType int

const (
	ConsumeToken = iota
	CheckPw
)

type AuthCommandError int

const (
	NoError = iota
	InvalidPassword
	TokenGeneration
	UnknownCommand
)

type AuthCommandResponse struct {
	authorized bool
	token      string
	user       string
	err        AuthCommandError
}

type AuthCommand struct {
	typ      AuthCommandType
	token    string
	user     string
	password string
	reply    chan AuthCommandResponse
}

type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func startAuthService(password string) (chan<- AuthCommand, error) {
	// set doesn't need to be synchronized, only the below goroutine will access it
	tokens := make(map[string]string)

	commands := make(chan AuthCommand)
	serverPw, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return nil, err
	}

	generateToken := func(user string) (string, error) {
		bytes := make([]byte, 32)
		_, err := rand.Read(bytes)
		if err != nil {
			return "", err
		}
		token := fmt.Sprintf("%x", bytes)
		tokens[token] = user
		return token, nil
	}

	go func() {
		for cmd := range commands {
			switch cmd.typ {
			case CheckPw:
				err := bcrypt.CompareHashAndPassword(serverPw, []byte(cmd.password))
				if err != nil {
					cmd.reply <- AuthCommandResponse{err: InvalidPassword}
					break
				}
				token, err := generateToken(cmd.user)
				if err != nil {
					cmd.reply <- AuthCommandResponse{err: TokenGeneration}
				} else {
					cmd.reply <- AuthCommandResponse{token: token}
				}
			case ConsumeToken:
				user, ok := tokens[cmd.token]
				if ok {
					delete(tokens, cmd.token)
					cmd.reply <- AuthCommandResponse{authorized: true, user: user}
				} else {
					cmd.reply <- AuthCommandResponse{authorized: true} // TODO: false
				}
			default:
				cmd.reply <- AuthCommandResponse{err: UnknownCommand}
			}
		}
	}()
	fmt.Println("started auth service")
	return commands, nil
}
