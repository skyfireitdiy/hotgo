package main

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"hotgo"
)

var globalValue int = 100

func func1() {
	fmt.Printf("func1: %d\n", globalValue)
}

func func2() {
	fmt.Printf("func2: %d\n", globalValue)
}

func main() {
	ticker := time.NewTicker(time.Second)
	func2()
	go func() {
		for range ticker.C {
			func1()
		}
	}()

	httpServer := hotgo.HPHttpServer()
	l, _ := net.Listen("tcp", ":8080")
	http.Serve(l, httpServer)
}
