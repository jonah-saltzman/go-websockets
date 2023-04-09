package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jonah-saltzman/go-websockets/server"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: go-websockets [port] [password]")
	}
	err := startHttpServer(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
}

func startHttpServer(port string, password string) error {
	fmt.Printf("starting server on port %s\n", port)
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return err
	}
	server, err := server.CreateServer(password)
	if err != nil {
		return err
	}
	httpServer := http.Server{
		Handler:      server,
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 10,
	}
	errorChannel := make(chan error)
	go func() {
		fmt.Println("listening")
		errorChannel <- httpServer.Serve(listener)
	}()
	for err := range errorChannel {
		fmt.Println(err)
	}
	return nil
}
