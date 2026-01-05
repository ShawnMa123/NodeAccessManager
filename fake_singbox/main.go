package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	configPath := flag.String("c", "", "config path")
	flag.Parse()

	fmt.Printf("Fake sing-box started with config: %s\n", *configPath)

	// Listen on port 10000 to simulate a reality node
	l, err := net.Listen("tcp", ":10000")
	if err != nil {
		fmt.Printf("Failed to listen: %v\n", err)
		os.Exit(1)
	}
	defer l.Close()

	fmt.Println("Listening on :10000")

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Printf("Accept error: %v\n", err)
			continue
		}
		fmt.Printf("New connection from %s\n", conn.RemoteAddr())
		go func(c net.Conn) {
			defer c.Close()
			time.Sleep(10 * time.Second) // Hold connection
		}(conn)
	}
}
