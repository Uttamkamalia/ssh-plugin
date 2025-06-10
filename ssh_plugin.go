package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/pebbe/zmq4"
	"log"
	"os"
	"strings"
	"time"
)

var config GlobalConfig = DefaultConfig()

type GlobalConfig struct {
	Concurrency   int           `json:"concurrency,omitempty"`
	DeviceTimeout time.Duration `json:"device_timeout,omitempty"`

	PingTimeout time.Duration `json:"ping_timeout,omitempty"`
	PortTimeout time.Duration `json:"port_timeout,omitempty"`
	SSHTimeout  time.Duration `json:"ssh_timeout,omitempty"`

	PingRetries int `json:"ping_retries,omitempty"`
	PortRetries int `json:"port_retries,omitempty"`
	SSHRetries  int `json:"ssh_retries,omitempty"`

	RetryBackoff time.Duration `json:"retry_backoff,omitempty"`

	// ZMQ configuration
	ZMQEndpoint            string        `json:"zmq_endpoint,omitempty"`
	ZMQPublisherWaitTimeMs time.Duration `json:"zmq_publisher_wait_time_ms,omitempty"`
}

func DefaultConfig() GlobalConfig {
	return GlobalConfig{
		Concurrency:            100,
		DeviceTimeout:          30 * time.Second,
		PingTimeout:            2 * time.Second,
		PortTimeout:            2 * time.Second,
		SSHTimeout:             5 * time.Second,
		PingRetries:            2,
		PortRetries:            2,
		SSHRetries:             2,
		RetryBackoff:           500 * time.Millisecond,
		ZMQEndpoint:            "tcp://127.0.0.1:5555",
		ZMQPublisherWaitTimeMs: 5000 * time.Millisecond,
	}
}

type SSHInput struct {
	DiscoveryProfileId  int           `json:"discovery_profile_id"`
	DiscoveryBatchJobId string        `json:"discovery_batch_job_id"`
	MetricIDs           []string      `json:"metric_ids"`
	Devices             []Device      `json:"devices"`
	Config              *GlobalConfig `json:"config,omitempty"`
}

type Device struct {
	DeviceTypeId  int `json:"device_type_id"`
	DeviceId      int `json:"device_id"`
	MetricGroupId int `json:"metric_group_id"`

	IP         string            `json:"ip"`
	Port       int               `json:"port"`
	Protocol   string            `json:"protocol"`
	Credential map[string]string `json:"credential"`
}

type SuccessfulResult struct {
	DeviceTypeId  int `json:"device_type_id,omitempty"`
	DeviceId      int `json:"device_id,omitempty"`
	MetricGroupId int `json:"metric_group_id,omitempty"`

	IP       string            `json:"ip"`
	Port     int               `json:"port"`
	Protocol string            `json:"protocol"`
	Metrics  map[string]string `json:"metrics"`
}

type FailedResult struct {
	DeviceTypeId  int `json:"device_type_id,omitempty"`
	DeviceId      int `json:"device_id,omitempty"`
	MetricGroupId int `json:"metric_group_id,omitempty"`

	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Error    string `json:"error"`
}

type ResultOutput struct {
	Successful []SuccessfulResult `json:"successful,omitempty"`
	Failed     []FailedResult     `json:"failed,omitempty"`
}

func main() {
	flag.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s DISCOVERY <input.json> <output.json>\n", os.Args[0])
		fmt.Printf("  %s POLLING <json-string>\n", os.Args[0])
	}
	flag.Parse()

	if len(os.Args) < 3 {
		flag.Usage()
		os.Exit(1)
	}

	mode := strings.ToUpper(os.Args[1])

	switch mode {
	case "DISCOVERY":
		if len(os.Args) < 4 {
			log.Fatalf("DISCOVERY mode requires input and output file paths")
		}
		runDiscovery(os.Args[2], os.Args[3])
	case "POLLING":
		runPolling(os.Args[2])
	default:
		log.Fatalf("Invalid MODE: %s (must be DISCOVERY or POLLING)", mode)
	}
}

func runDiscovery(inputFile, outputFile string) {
	raw, err := os.ReadFile(inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var input SSHInput
	if err := json.Unmarshal(raw, &input); err != nil {
		log.Fatalf("Failed to parse input JSON: %v", err)
	}

	applyOptionalConfig(input.Config)

	resultsChan := make(chan ResultOutput, 100)
	doneChan := make(chan struct{})

	go writeResultsToFile(outputFile, resultsChan, doneChan)

	processDevicesConcurrently(input, "DISCOVERY", resultsChan)

	close(resultsChan)
	<-doneChan
}

func runPolling(rawJSON string) {
	var input SSHInput
	if err := json.Unmarshal([]byte(rawJSON), &input); err != nil {
		log.Fatalf("Failed to parse input JSON: %v", err)
	}

	applyOptionalConfig(input.Config)

	resultsChan := make(chan ResultOutput, 100)
	doneChan := make(chan struct{})

	//go writeResultsToConsole(resultsChan, doneChan)
	go writeResultsToZMQ(resultsChan, doneChan, config.ZMQEndpoint, config.ZMQPublisherWaitTimeMs)

	processDevicesConcurrently(input, "POLLING", resultsChan)

	close(resultsChan)
	<-doneChan
}

func applyOptionalConfig(custom *GlobalConfig) {
	if custom == nil {
		return
	}

	if custom.Concurrency > 0 {
		config.Concurrency = custom.Concurrency
	}
	if custom.DeviceTimeout > 0 {
		config.DeviceTimeout = custom.DeviceTimeout * time.Second
	}
	if custom.PingTimeout > 0 {
		config.PingTimeout = custom.PingTimeout * time.Second
	}
	if custom.PortTimeout > 0 {
		config.PortTimeout = custom.PortTimeout * time.Second
	}
	if custom.SSHTimeout > 0 {
		config.SSHTimeout = custom.SSHTimeout * time.Second
	}
	if custom.PingRetries > 0 {
		config.PingRetries = custom.PingRetries
	}
	if custom.PortRetries > 0 {
		config.PortRetries = custom.PortRetries
	}
	if custom.SSHRetries > 0 {
		config.SSHRetries = custom.SSHRetries
	}
	if custom.RetryBackoff > 0 {
		config.RetryBackoff = custom.RetryBackoff * time.Millisecond
	}
	if custom.ZMQEndpoint != "" {
		config.ZMQEndpoint = custom.ZMQEndpoint
	}
	if custom.ZMQPublisherWaitTimeMs > 0 {
		config.ZMQPublisherWaitTimeMs = custom.ZMQPublisherWaitTimeMs * time.Millisecond
	}
}

func processDevicesConcurrently(input SSHInput, mode string, resultsChan chan<- ResultOutput) {
	sem := make(chan struct{}, config.Concurrency)

	for _, device := range input.Devices {
		sem <- struct{}{}

		go func(d Device) {
			defer func() { <-sem }()

			done := make(chan struct{})
			var sres SuccessfulResult
			var fres *FailedResult

			go func() {
				sres, fres = ProcessDeviceWithTimeout(d, input.MetricIDs, mode, config.DeviceTimeout)
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(config.DeviceTimeout):
				fres = &FailedResult{
					DeviceTypeId:  d.DeviceTypeId,
					DeviceId:      d.DeviceId,
					MetricGroupId: d.MetricGroupId,
					IP:            d.IP,
					Port:          d.Port,
					Protocol:      d.Protocol,
					Error:         "device-timeout",
				}
			}

			if fres != nil {
				resultsChan <- ResultOutput{Failed: []FailedResult{*fres}}
			} else {
				resultsChan <- ResultOutput{Successful: []SuccessfulResult{sres}}
			}
		}(device)
	}

	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
}

func writeResultsToFile(path string, resultsChan <-chan ResultOutput, doneChan chan<- struct{}) {
	var finalResult ResultOutput

	for result := range resultsChan {
		finalResult.Successful = append(finalResult.Successful, result.Successful...)
		finalResult.Failed = append(finalResult.Failed, result.Failed...)
	}

	file, err := os.Create(path)
	if err != nil {
		log.Fatalf("Cannot create output file: %v", err)
	}
	defer func() {
		file.Sync()
		file.Close()
		doneChan <- struct{}{}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty-print output

	if err := encoder.Encode(finalResult); err != nil {
		log.Printf("Failed to write final result: %v", err)
	}
}

func writeResultsToConsole(resultsChan <-chan ResultOutput, doneChan chan<- struct{}) {
	var finalResult ResultOutput

	for result := range resultsChan {
		finalResult.Successful = append(finalResult.Successful, result.Successful...)
		finalResult.Failed = append(finalResult.Failed, result.Failed...)
	}

	data, err := json.MarshalIndent(finalResult, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal final result: %v", err)
	} else {
		fmt.Println(string(data))
	}

	doneChan <- struct{}{}
}

func writeResultsToZMQ(resultsChan <-chan ResultOutput, doneChan chan<- struct{}, zmqEndpoint string, publisherWaitTimeMs time.Duration) {
	// Create a publisher socket
	publisher, err := zmq4.NewSocket(zmq4.PUSH)
	if err != nil {
		log.Fatalf("Failed to create ZMQ socket: %v", err)
	}
	defer publisher.Close()

	// Bind to the endpoint
	if err := publisher.Connect(zmqEndpoint); err != nil {
		log.Fatalf("Failed to bind ZMQ socket to %s: %v", zmqEndpoint, err)
	}
	time.Sleep(publisherWaitTimeMs)

	// Process and send each result as it arrives
	for result := range resultsChan {
		// Send successful results
		for _, success := range result.Successful {
			// Marshal each successful result individually
			data, err := json.Marshal(success)
			if err != nil {
				log.Printf("Failed to marshal successful result: %v", err)
				continue
			}

			// Publish with "success" topic
			if _, err := publisher.Send("success "+string(data), 0); err != nil {
				log.Printf("Failed to publish successful result: %v", err)
			}
		}
		log.Printf("sent data to publish successful result: %v to address %s", result, zmqEndpoint)

		// Send failed results
		for _, failure := range result.Failed {
			// Marshal each failed result individually
			data, err := json.Marshal(failure)
			if err != nil {
				log.Printf("Failed to marshal failed result: %v", err)
				continue
			}

			// Publish with "failure" topic
			if _, err := publisher.Send("failure "+string(data), 0); err != nil {
				log.Printf("Failed to publish failed result: %v", err)
			}
			log.Printf("sent data to publish failed result: %v to address %s", result, zmqEndpoint)
		}
	}
	doneChan <- struct{}{}
}
