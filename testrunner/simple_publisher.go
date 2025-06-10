package main

import (
	"fmt"
	"github.com/pebbe/zmq4"
	"time"
)

func main() {
	// Create publisher socket
	publisher, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		fmt.Printf("Error creating socket: %v\n", err)
		return
	}
	defer publisher.Close()

	// Bind to endpoint
	err = publisher.Bind("tcp://127.0.0.1:5555")
	if err != nil {
		fmt.Printf("Error binding: %v\n", err)
		return
	}

	fmt.Println("Publisher started on tcp://*:5555")
	fmt.Println("Waiting for subscribers to connect...")
	time.Sleep(2 * time.Second)

	// Publish messages
	counter := 0
	for {
		counter++

		// Send success message
		successMsg := fmt.Sprintf("success {\"id\":%d,\"status\":\"ok\",\"data\":\"test data %d\"}", counter, counter)
		publisher.Send(successMsg, 0)
		fmt.Printf("Published: %s\n", successMsg)

		// Send failure message
		failureMsg := fmt.Sprintf("failure {\"id\":%d,\"status\":\"error\",\"reason\":\"test error %d\"}", counter, counter)
		publisher.Send(failureMsg, 0)
		fmt.Printf("Published: %s\n", failureMsg)

		time.Sleep(1 * time.Second)
	}
}
