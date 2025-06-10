package main

import (
    "fmt"
    "github.com/pebbe/zmq4"
    "strings"
)

func main() {
    // Create subscriber socket
    subscriber, err := zmq4.NewSocket(zmq4.SUB)
    if err != nil {
        fmt.Printf("Error creating socket: %v\n", err)
        return
    }
    defer subscriber.Close()

    // Connect to publisher
    err = subscriber.Connect("tcp://127.0.0.1:5555")
    if err != nil {
        fmt.Printf("Error connecting: %v\n", err)
        return
    }

    // Subscribe to all topics (empty string means all)
    subscriber.SetSubscribe("")
    
    fmt.Println("Subscriber connected to tcp://127.0.0.1:5555")
    fmt.Println("Listening for messages...")

    // Receive messages
    for {
        message, err := subscriber.Recv(0)
        if err != nil {
            fmt.Printf("Error receiving: %v\n", err)
            continue
        }

        // Parse topic and content
        parts := strings.SplitN(message, " ", 2)
        if len(parts) == 2 {
            topic := parts[0]
            content := parts[1]
            fmt.Printf("Received [%s]: %s\n", topic, content)
        } else {
            fmt.Printf("Received: %s\n", message)
        }
    }
}