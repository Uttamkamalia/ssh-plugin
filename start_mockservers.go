package main

import (
	"crypto/rand"
	"crypto/rsa"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var excludedIPs = map[string]bool{}

func main() {
	username := "username"
	password := "password"
	port := 2222
	count := 0

	// Parse optional flag to exclude IPs
	excludeList := flag.String("exclude", "", "Comma-separated list of IPs to exclude")
	flag.Parse()

	if *excludeList != "" {
		for _, ip := range strings.Split(*excludeList, ",") {
			excludedIPs[strings.TrimSpace(ip)] = true
		}
	}

	for i := 0; i < 4; i++ {
		for j := 1; j <= 254; j++ {
			ip := fmt.Sprintf("127.10.%d.%d", i, j)

			if excludedIPs[ip] {
				log.Printf("🚫 Skipping excluded IP %s", ip)
				continue
			}

			startMockSSH(ip, port, username, password)
			count++
			if count >= 1024 {
				break
			}
			time.Sleep(3 * time.Millisecond)
		}
		if count >= 1024 {
			break
		}
	}

	fmt.Println("🚀 All mock SSH servers started (excluding specified IPs)")
	select {} // Block forever
}

func startMockSSH(ip string, port int, username, password string) {
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("unauthorized")
		},
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("Failed to generate RSA key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		log.Fatalf("Failed to create signer: %v", err)
	}
	config.AddHostKey(signer)

	addr := fmt.Sprintf("%s:%d", ip, port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("⚠️ Failed to listen on %s: %v", addr, err)
		return
	}
	log.Printf("✅ Mock SSH server on %s", addr)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			go handleConnection(conn, config)
		}
	}()
}

func handleConnection(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs)

	for ch := range chans {
		if ch.ChannelType() != "session" {
			ch.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go func(in <-chan *ssh.Request) {
			for req := range in {
				if req.Type == "exec" {
					req.Reply(true, nil)
					cmd := string(req.Payload[4:])
					var response string

					switch {
					case strings.Contains(cmd, "Cpu(s)"):
						response = "15.5"
					case strings.Contains(cmd, "free") && strings.Contains(cmd, "awk"):
						response = "68.23"
					case strings.Contains(cmd, "df /"):
						response = "42%"
					case strings.Contains(cmd, "uptime"):
						response = "up 1 hour, 32 minutes"
					case strings.Contains(cmd, "uname"):
						response = "Linux mock-host 5.15.0-virtual #1 SMP"
					default:
						response = "mock-output-for: " + cmd
					}

					channel.Write([]byte(response))
					channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					channel.Close()
				}
			}
		}(requests)
	}
}
