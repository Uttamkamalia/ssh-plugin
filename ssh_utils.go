package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-ping/ping"
	"github.com/pebbe/zmq4"
)

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
	ZMQHighWaterMark       int           `json:"zmq_high_watermark,omitempty"`
}

type SSHInput struct {
	DiscoveryProfileId int           `json:"discovery_profile_id"` // TODO DiscoveryProfileId can be made general-id
	JobId              string        `json:"job_id"`
	DeviceTypeId       int           `json:"device_type_id"`
	MetricGroupId      int           `json:"metric_group_id"`
	MetricIDs          []string      `json:"metric_ids"`
	Devices            []Device      `json:"devices"`
	Config             *GlobalConfig `json:"config,omitempty"`
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

var MetricCommandMap = map[string]string{
	"cpu_usage":    `top -bn1 | grep "Cpu(s)" | awk '{print $2 + $4}'`,
	"memory_usage": `free | awk '/Mem:/ { printf("%.2f", $3/$2 * 100.0) }'`,
	"disk_usage":   `df / | awk 'NR==2 { print $5 }'`,
	"uptime":       `uptime -p`,
	"uname":        `uname -a`,
	"up":           `echo "up"`,
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

func Failed(device Device, reason string) (SuccessfulResult, *FailedResult) {
	return SuccessfulResult{}, &FailedResult{
		DeviceTypeId:  device.DeviceTypeId,
		DeviceId:      device.DeviceId,
		MetricGroupId: device.MetricGroupId,
		IP:            device.IP,
		Port:          device.Port,
		Protocol:      device.Protocol,
		Error:         reason,
	}
}

func Ping(ip string) (bool, error) {
	var lastErr error
	for i := 0; i < config.PingRetries; i++ {
		pinger, err := ping.NewPinger(ip)
		if err != nil {
			lastErr = fmt.Errorf("create pinger: %w", err)
			time.Sleep(config.RetryBackoff)
			continue
		}
		pinger.Count = 3
		pinger.Timeout = config.PingTimeout
		pinger.SetPrivileged(true)

		if err := pinger.Run(); err != nil {
			lastErr = fmt.Errorf("ping run failed: %w", err)
			time.Sleep(config.RetryBackoff)
			continue
		}

		stats := pinger.Statistics()
		if stats.PacketsRecv == 0 {
			lastErr = fmt.Errorf("no packets received")
			time.Sleep(config.RetryBackoff)
			continue
		}
		return true, nil
	}
	return false, lastErr
}

func PortOpen(ip string, port int) (bool, error) {
	var lastErr error
	for i := 0; i < config.PortRetries; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), config.PortTimeout)
		if err == nil {
			conn.Close()
			return true, nil
		}
		lastErr = fmt.Errorf("dial timeout: %w", err)
		time.Sleep(config.RetryBackoff)
	}
	return false, lastErr
}

func writeResultsToFile(path string, successChan <-chan SuccessfulResult, failedChan <-chan *FailedResult, doneChan chan<- struct{}) {
	var finalResult ResultOutput
	successChanClosed := false
	failedChanClosed := false

	for {
		if successChanClosed && failedChanClosed {
			break
		}

		select {
		case success, ok := <-successChan:
			if !ok {
				successChanClosed = true
				continue
			}
			finalResult.Successful = append(finalResult.Successful, success)

		case failed, ok := <-failedChan:
			if !ok {
				failedChanClosed = true
				continue
			}
			finalResult.Failed = append(finalResult.Failed, *failed)
		}
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

func writeResultsToZMQ(
	successChan <-chan SuccessfulResult,
	failedChan <-chan *FailedResult,
	doneChan chan<- struct{},
	zmqEndpoint string,
	publisherWaitTimeMs time.Duration,
	hwm int,
) {
	publisher, err := zmq4.NewSocket(zmq4.PUSH)
	if err != nil {
		log.Fatalf("Failed to create ZMQ socket: %v", err)
	}

	defer func() {
		err := publisher.Close()
		if err != nil {
			log.Printf("Failed to close ZMQ socket: %v", err)
		}
		doneChan <- struct{}{}
	}()

	if err := publisher.SetSndhwm(hwm); err != nil {
		log.Printf("Warning: Failed to set send HWM: %v", err)
	}

	if err := publisher.Connect(zmqEndpoint); err != nil {
		log.Fatalf("Failed to connect ZMQ socket to %s: %v", zmqEndpoint, err)
	}

	time.Sleep(publisherWaitTimeMs)

	successChanClosed := false
	failedChanClosed := false

	for !(successChanClosed && failedChanClosed) {
		select {
		case success, ok := <-successChan:
			if !ok {
				successChanClosed = true
				continue
			}
			sendZMQMessage(publisher, "success", success, zmqEndpoint)

		case failure, ok := <-failedChan:
			if !ok {
				failedChanClosed = true
				continue
			}
			sendZMQMessage(publisher, "failure", failure, zmqEndpoint)
		}
	}
}

func sendZMQMessage(publisher *zmq4.Socket, topic string, payload interface{}, endpoint string) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal %s result: %v", topic, err)
		return
	}

	message := topic + " " + string(data)
	if _, err := publisher.Send(message, 0); err != nil {
		log.Printf("Failed to publish %s result: %v", topic, err)
	} else {
		log.Printf("Sent %s result: %v to %s", topic, payload, endpoint)
	}
}

// backup
func writeResultsToFileBackup(path string, resultsChan <-chan ResultOutput, doneChan chan<- struct{}) {
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

func writeResultsToZMQBackup(resultsChan <-chan ResultOutput, doneChan chan<- struct{}, zmqEndpoint string, publisherWaitTimeMs time.Duration, hwm int) {
	// Create a publisher socket
	publisher, err := zmq4.NewSocket(zmq4.PUSH)
	if err != nil {
		log.Fatalf("Failed to create ZMQ socket: %v", err)
	}
	defer publisher.Close()

	// Set high water mark (HWM) - maximum number of messages queued
	if err := publisher.SetSndhwm(hwm); err != nil {
		log.Printf("Warning: Failed to set send HWM: %v", err)
	}
	log.Printf("ZMQ send high water mark set to %d messages", hwm)

	if err := publisher.Connect(zmqEndpoint); err != nil {
		log.Fatalf("Failed to connect ZMQ socket to %s: %v", zmqEndpoint, err)
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
