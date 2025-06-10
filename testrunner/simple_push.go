package main

import (
	"fmt"
	"github.com/pebbe/zmq4"
	"time"
)

func main() {
	// Create PUSH socket
	pusher, err := zmq4.NewSocket(zmq4.PUSH)
	if err != nil {
		fmt.Printf("Error creating socket: %v\n", err)
		return
	}
	defer pusher.Close()

	// Bind to endpoint
	err = pusher.Bind("tcp://localhost:5555")
	if err != nil {
		fmt.Printf("Error binding: %v\n", err)
		return
	}

	fmt.Println("PUSH socket bound to tcp://*:5555")
	fmt.Println("Waiting for receivers to connect...")
	time.Sleep(2 * time.Second)

	// Send messages
	counter := 0
	for {
		counter++
		msg := fmt.Sprintf("Message #%d from PUSH socket", counter)
		_, err := pusher.Send(msg, 0)
		if err != nil {
			fmt.Printf("Error sending: %v\n", err)
		} else {
			fmt.Printf("Sent: %s\n", msg)
		}
		time.Sleep(1 * time.Second)
	}
}
