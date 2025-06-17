package main

import (
	"fmt"
	"github.com/pebbe/zmq4"
	"time"
)

func main() {
	// Create PULL socket
	puller, err := zmq4.NewSocket(zmq4.PULL)
	if err != nil {
		fmt.Printf("Error creating socket: %v\n", err)
		return
	}
	defer puller.Close()

	// Connect to endpoint
	err = puller.Bind("tcp://localhost:5555")
	if err != nil {
		fmt.Printf("Error connecting: %v\n", err)
		return
	}

	fmt.Println("PULL socket connected to tcp://localhost:5555")
	fmt.Println("Listening for messages...")

	// Receive messages
	var count = 0
	var startTime = time.Now().Unix()
	for {
		message, err := puller.Recv(0)
		if err != nil {
			fmt.Printf("Error receiving: %v\n", err)
			continue
		}
		fmt.Printf("Received: %s total-messages received: %d at time:  %d secs\n", message, count, time.Now().Unix()-startTime)
		count = count + 1
	}
}
