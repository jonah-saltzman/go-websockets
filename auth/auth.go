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
const SALT_BYTES int = 4

type AuthCommandType int

const (
	CheckToken = iota
	CreateToken
	ConsumeToken
)

type AuthCommand struct {
	Typ      AuthCommandType
	Token    string
	User     *User
	Password string
	Reply    chan AuthCommandResponse
}

type AuthCommandError int

const (
	NoError = iota
	InvalidPassword
	TokenGenerationErr
	UnknownCommand
)

type AuthCommandResponse struct {
	Token string
	User  *User
	Err   AuthCommandError
}

type LoginRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type User struct {
	Id   uuid.UUID    `json:"id"`
	Name string       `json:"name"`
	Out  chan *[]byte `json:"-"`
}

type Token struct {
	User *User     `json:"user"`
	Exp  time.Time `json:"exp"`
}

type TokenContainer struct {
	tokens   map[string]*Token
	serverPw []byte
	salt     []byte
}

// auth service is a goroutine which handles logins and authentication of websocket
// requests. If a user provides the correct server password, they receive a token
// which can be used to join the chat room and request the message history.
func StartAuthService(password string) (chan<- AuthCommand, error) {

	commands := make(chan AuthCommand, AUTH_CHAN_DEPTH)
	salt := make([]byte, SALT_BYTES)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	salted := append([]byte(password), salt...)
	serverPw, err := bcrypt.GenerateFromPassword(salted, 7)
	if err != nil {
		return nil, err
	}

	// container doesn't need to be synchronized, only the below goroutine will access it
	tc := TokenContainer{tokens: make(map[string]*Token), serverPw: serverPw, salt: salt}

	go func() {
		for cmd := range commands {
			switch cmd.Typ {
			case CreateToken:
				token, err := tc.generateToken(cmd.User, []byte(cmd.Password))
				cmd.Reply <- AuthCommandResponse{Token: token, Err: err}
			case CheckToken:
				user := tc.checkToken(cmd.Token)
				cmd.Reply <- AuthCommandResponse{User: user}
			case ConsumeToken:
				user := tc.consumeToken(cmd.Token)
				cmd.Reply <- AuthCommandResponse{User: user}
			default:
				cmd.Reply <- AuthCommandResponse{Err: UnknownCommand}
			}
		}
	}()
	return commands, nil
}

func (tc *TokenContainer) generateToken(user *User, password []byte) (string, AuthCommandError) {
	err := bcrypt.CompareHashAndPassword(tc.serverPw, append(password, tc.salt...))
	if err != nil {
		return "", InvalidPassword
	}
	exp := time.Now().Add(TOKEN_EXP)
	bytes := make([]byte, TOKEN_BYTES)
	_, err = rand.Read(bytes)
	if err != nil {
		return "", TokenGenerationErr
	}
	token := fmt.Sprintf("%x", bytes)
	tc.tokens[token] = &Token{User: user, Exp: exp}
	return token, NoError
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
