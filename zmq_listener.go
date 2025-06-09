package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/pebbe/zmq4"
)

func main() {
    // Parse command line arguments
    endpoint := flag.String("endpoint", "tcp://127.0.0.1:5555", "ZMQ endpoint to subscribe to")
    topics := flag.String("topics", "", "Topics to subscribe to (empty for all)")
    flag.Parse()

    fmt.Printf("Listening for messages on %s\n", *endpoint)
    if *topics != "" {
        fmt.Printf("Subscribed to topics: %s\n", *topics)
    } else {
        fmt.Println("Subscribed to all topics")
    }

    // Create a subscriber socket
    subscriber, err := zmq4.NewSocket(zmq4.SUB)
    if err != nil {
        log.Fatalf("Failed to create ZMQ socket: %v", err)
    }
    defer subscriber.Close()

    // Connect to the endpoint
    if err := subscriber.Connect(*endpoint); err != nil {
        log.Fatalf("Failed to connect to %s: %v", *endpoint, err)
    }

    // Set subscription filter
    if *topics == "" {
        // Subscribe to all messages
        subscriber.SetSubscribe("")
    } else {
        // Subscribe to specific topics
        subscriber.SetSubscribe(*topics)
    }

    // Handle Ctrl+C gracefully
    signalChan := make(chan os.Signal, 1)
    signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-signalChan
        fmt.Println("\nShutting down...")
        subscriber.Close()
        os.Exit(0)
    }()

    fmt.Println("Waiting for messages... (Press Ctrl+C to quit)")

    // Receive and print messages
    for {
        // Receive message
        msg, err := subscriber.Recv(0)
        if err != nil {
            log.Printf("Error receiving message: %v", err)
            continue
        }

        // Print the message
        fmt.Printf("Received: %s\n", msg)
    }
}