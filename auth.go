package main

import (
	"crypto/rand"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const TOKEN_EXP time.Duration = time.Hour * 24

type AuthCommandType int

const (
	CheckToken = iota
	CreateToken
	ConsumeToken
)

type AuthCommandError int

const (
	NoAuthError = iota
	InvalidPassword
	TokenGeneration
	UnknownCommand
)

type AuthCommandResponse struct {
	authorized bool
	token      string
	user       *User
	err        AuthCommandError
}

type AuthCommand struct {
	typ      AuthCommandType
	token    string
	user     *User
	password string
	reply    chan AuthCommandResponse
}

type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type Token struct {
	User *User     `json:"user"`
	Exp  time.Time `json:"exp"`
}

type TokenContainer struct {
	tokens map[string]*Token
}

func (c *TokenContainer) generateToken(user *User) (string, error) {
	exp := time.Now().Add(TOKEN_EXP)
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	token := fmt.Sprintf("%x", bytes)
	c.tokens[token] = &Token{User: user, Exp: exp}
	return token, nil
}

func (c *TokenContainer) checkToken(tokenString string) *User {
	token, ok := c.tokens[tokenString]
	if ok {
		if token.Exp.After(time.Now()) {
			return token.User
		} else {
			delete(c.tokens, tokenString)
			return nil
		}
	}
	return nil
}

func (c *TokenContainer) consumeToken(tokenString string) *User {
	exists := c.checkToken(tokenString)
	if exists != nil {
		delete(c.tokens, tokenString)
	}
	return exists
}

func startAuthService(password string) (chan<- AuthCommand, error) {
	// container doesn't need to be synchronized, only the below goroutine will access it
	tc := TokenContainer{tokens: make(map[string]*Token)}

	commands := make(chan AuthCommand)
	serverPw, err := bcrypt.GenerateFromPassword([]byte(password), 7)
	if err != nil {
		return nil, err
	}

	go func() {
		for cmd := range commands {
			switch cmd.typ {
			case CreateToken:
				err := bcrypt.CompareHashAndPassword(serverPw, []byte(cmd.password))
				if err != nil {
					cmd.reply <- AuthCommandResponse{err: InvalidPassword}
					break
				}
				token, err := tc.generateToken(cmd.user)
				if err != nil {
					cmd.reply <- AuthCommandResponse{err: TokenGeneration}
				} else {
					cmd.reply <- AuthCommandResponse{token: token}
				}
			case CheckToken:
				user := tc.checkToken(cmd.token)
				respondGetToken(user, cmd.reply)
			case ConsumeToken:
				user := tc.consumeToken(cmd.token)
				respondGetToken(user, cmd.reply)
			default:
				cmd.reply <- AuthCommandResponse{err: UnknownCommand}
			}
		}
	}()
	fmt.Println("started auth service")
	return commands, nil
}

func respondGetToken(user *User, c chan AuthCommandResponse) {
	if user != nil {
		c <- AuthCommandResponse{authorized: true, user: user}
	} else {
		c <- AuthCommandResponse{authorized: false, user: nil}
	}
}
