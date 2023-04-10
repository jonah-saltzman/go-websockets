package auth

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const TOKEN_EXP time.Duration = time.Hour * 24
const AUTH_CHAN_DEPTH int = 100
const TOKEN_BYTES int = 32

type AuthCommandType int

const (
	CheckToken = iota
	CreateToken
	ConsumeToken
)

type AuthCommandError int

const (
	NoError = iota
	InvalidPassword
	TokenGeneration
	UnknownCommand
)

type AuthCommandResponse struct {
	Authorized bool
	Token      string
	User       *User
	Err        AuthCommandError
}

type AuthCommand struct {
	Typ      AuthCommandType
	Token    string
	User     *User
	Password string
	Reply    chan AuthCommandResponse
}

type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type User struct {
	Id   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Out chan *[]byte `json:"-"`
}

// auth service is a goroutine which handles logins and authentication of websocket
// requests. If a user provides the correct server password, they receive a token
// which can be used to join the chat room and request the message history.
func StartAuthService(password string) (chan<- AuthCommand, error) {
	// container doesn't need to be synchronized, only the below goroutine will access it
	tc := TokenContainer{tokens: make(map[string]*Token)}

	commands := make(chan AuthCommand, AUTH_CHAN_DEPTH)
	serverPw, err := bcrypt.GenerateFromPassword([]byte(password), 7)
	if err != nil {
		return nil, err
	}

	go func() {
		for cmd := range commands {
			switch cmd.Typ {
			case CreateToken:
				err := bcrypt.CompareHashAndPassword(serverPw, []byte(cmd.Password))
				if err != nil {
					cmd.Reply <- AuthCommandResponse{Err: InvalidPassword}
					break
				}
				token, err := tc.generateToken(cmd.User)
				if err != nil {
					cmd.Reply <- AuthCommandResponse{Err: TokenGeneration}
				} else {
					cmd.Reply <- AuthCommandResponse{Token: token}
				}
			case CheckToken:
				user := tc.checkToken(cmd.Token)
				respondGetToken(user, cmd.Reply)
			case ConsumeToken:
				user := tc.consumeToken(cmd.Token)
				respondGetToken(user, cmd.Reply)
			default:
				cmd.Reply <- AuthCommandResponse{Err: UnknownCommand}
			}
		}
	}()
	return commands, nil
}

func respondGetToken(user *User, c chan AuthCommandResponse) {
	if user != nil {
		c <- AuthCommandResponse{Authorized: true, User: user}
	} else {
		c <- AuthCommandResponse{Authorized: false, User: nil}
	}
}

type Token struct {
	User *User     `json:"user"`
	Exp  time.Time `json:"exp"`
}

type TokenContainer struct {
	tokens map[string]*Token
}

func (tc *TokenContainer) generateToken(user *User) (string, error) {
	exp := time.Now().Add(TOKEN_EXP)
	bytes := make([]byte, TOKEN_BYTES)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	token := fmt.Sprintf("%x", bytes)
	tc.tokens[token] = &Token{User: user, Exp: exp}
	return token, nil
}

func (tc *TokenContainer) checkToken(tokenString string) *User {
	token, ok := tc.tokens[tokenString]
	if ok {
		if token.Exp.After(time.Now()) {
			return token.User
		} else {
			delete(tc.tokens, tokenString)
			return nil
		}
	}
	return nil
}

func (tc *TokenContainer) consumeToken(tokenString string) *User {
	exists := tc.checkToken(tokenString)
	if exists != nil {
		delete(tc.tokens, tokenString)
	}
	return exists
}
