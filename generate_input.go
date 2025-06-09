package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Device struct {
	IP       string            `json:"ip"`
	Port     int               `json:"port"`
	Protocol string            `json:"protocol"`
	Creds    map[string]string `json:"creds"`
}

type SSHInput struct {
	Devices []Device `json:"devices"`
}

func main() {
	devices := make([]Device, 0, 1024)

	// ipPrefix := [4]int{127, 10, 0, 1}
	count := 0

	for i := 0; i < 4; i++ {
		for j := 1; j <= 254; j++ {
			ip := fmt.Sprintf("127.10.%d.%d", i, j)
			device := Device{
				IP:       ip,
				Port:     2222,
				Protocol: "ssh",
				Creds: map[string]string{
					"username": "testuser",
					"password": "testpass",
				},
			}
			devices = append(devices, device)
			count++
			if count >= 1024 {
				break
			}
		}
		if count >= 1024 {
			break
		}
	}

	input := SSHInput{Devices: devices}
	file, err := os.Create("input.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	err = enc.Encode(input)
	if err != nil {
		panic(err)
	}

	fmt.Println("✅ input.json generated with 1024 mock SSH devices")
}
