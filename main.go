package main

import (
	// "context"
	// "errors"
	// "log"
	// "sync"
	"fmt"
	"net"
	"net/http"
	"time"
	// "nhooyr.io/websocket"
)

func main() {
	err := startHttpServer(8080)
	if err != nil {
		fmt.Println(err)
	}
}

func startHttpServer(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	server := createServer()
	httpServer := http.Server{
		Handler:      server,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- httpServer.Serve(listener)
	}()
	for err := range errorChannel {
		fmt.Println(err)
	}
	return nil
}
