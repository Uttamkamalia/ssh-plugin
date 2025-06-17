package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"os"
	"strings"
	"time"
)

const (
	DISCOVERY = "DISCOVERY"
	POLLING   = "POLLING"
)

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
		ZMQPublisherWaitTimeMs: 5 * time.Second, // Wait until the pull socket is connected before starting to push
		ZMQHighWaterMark:       1000,
	}
}

var config GlobalConfig = DefaultConfig()

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
	// TODO define DISCOVERY, POLLING as enum if possible
	case DISCOVERY:
		if len(os.Args) < 4 {
			log.Fatalf("DISCOVERY mode requires input and output file paths")
		}
		inputFilePath := os.Args[2]
		outputFilePath := os.Args[3]
		runDiscovery(inputFilePath, outputFilePath)
	case POLLING:
		jsonInput := os.Args[2]
		runPolling(jsonInput)
	default:
		log.Fatalf("Invalid MODE: %s (must be DISCOVERY or POLLING)", mode)
	}
}

func runDiscovery(inputFilePath, outputFilePath string) {
	raw, err := os.ReadFile(inputFilePath)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	var input SSHInput
	if err := json.Unmarshal(raw, &input); err != nil {
		log.Fatalf("Failed to parse input JSON: %v", err)
	}

	applyOptionalConfig(input.Config)

	successChan := make(chan SuccessfulResult, config.Concurrency)
	failedChan := make(chan *FailedResult, config.Concurrency)
	doneChan := make(chan struct{})

	go writeResultsToFile(outputFilePath, successChan, failedChan, doneChan)

	processDevicesConcurrently(input, DISCOVERY, successChan, failedChan)

	close(successChan)
	close(failedChan)
	<-doneChan
}

func runPolling(rawJSON string) {
	var input SSHInput
	if err := json.Unmarshal([]byte(rawJSON), &input); err != nil {
		log.Fatalf("Failed to parse input JSON: %v", err)
	}

	applyOptionalConfig(input.Config)

	successChan := make(chan SuccessfulResult, config.Concurrency)
	failedChan := make(chan *FailedResult, config.Concurrency)
	doneChan := make(chan struct{})

	go writeResultsToZMQ(successChan, failedChan, doneChan, config.ZMQEndpoint, config.ZMQPublisherWaitTimeMs, config.ZMQHighWaterMark)

	processDevicesConcurrently(input, POLLING, successChan, failedChan)

	close(successChan)
	close(failedChan)
	<-doneChan
}

func processDevicesConcurrently(input SSHInput, mode string, successChan chan<- SuccessfulResult, failedChan chan<- *FailedResult) {
	concurrentDevicesSemaphore := make(chan struct{}, config.Concurrency)

	for _, device := range input.Devices {
		concurrentDevicesSemaphore <- struct{}{}
		go ProcessDeviceWithTimeout(device, input.MetricIDs, mode, config.DeviceTimeout, concurrentDevicesSemaphore, successChan, failedChan)
	}

	for i := 0; i < cap(concurrentDevicesSemaphore); i++ {
		concurrentDevicesSemaphore <- struct{}{}
	}
}

func ProcessDeviceWithTimeout(device Device, metricIDs []string, mode string, timeout time.Duration, semaphoreChan chan struct{}, successChan chan<- SuccessfulResult, failedChan chan<- *FailedResult) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in device processing: %v", r)
			failedChan <- &FailedResult{
				DeviceTypeId:  device.DeviceTypeId,
				DeviceId:      device.DeviceId,
				MetricGroupId: device.MetricGroupId,
				IP:            device.IP,
				Port:          device.Port,
				Protocol:      device.Protocol,
				Error:         fmt.Sprintf("internal-error: panic recovered: %v", r),
			}
		}
		<-semaphoreChan
		cancel()
	}()

	successResult, failedResult := ProcessDevice(device, metricIDs, mode)

	select {
	case <-ctx.Done():
		failedChan <- &FailedResult{
			DeviceTypeId:  device.DeviceTypeId,
			DeviceId:      device.DeviceId,
			MetricGroupId: device.MetricGroupId,
			IP:            device.IP,
			Port:          device.Port,
			Protocol:      device.Protocol,
			Error:         "device-timeout",
		}
	default:
		if failedResult != nil {
			failedChan <- failedResult
		} else {
			successChan <- successResult
		}
	}

}

func ProcessDevice(device Device, metricIDs []string, mode string) (SuccessfulResult, *FailedResult) {
	if mode == DISCOVERY {
		if ok, err := Ping(device.IP); !ok {
			return Failed(device, "ping-check-failed: "+err.Error())
		}
		if ok, err := PortOpen(device.IP, device.Port); !ok {
			return Failed(device, "port-check-failed: "+err.Error())
		}
	}

	username := device.Credential["username"]
	password := device.Credential["password"]
	metrics := make(map[string]string)

	for _, metric := range metricIDs {
		cmd, ok := MetricCommandMap[strings.ToLower(metric)]
		if !ok {
			metrics[metric] = "unsupported-metric"
			continue
		}
		value, err := RunSSHCommand(device.IP, device.Port, username, password, cmd)
		if err != nil {
			metrics[metric] = "error: " + err.Error()
		} else {
			metrics[metric] = strings.TrimSpace(value)
		}
	}

	return SuccessfulResult{
		DeviceTypeId:  device.DeviceTypeId,
		DeviceId:      device.DeviceId,
		MetricGroupId: device.MetricGroupId,
		IP:            device.IP,
		Port:          device.Port,
		Protocol:      device.Protocol,
		Metrics:       metrics,
	}, nil
}

func RunSSHCommand(ip string, port int, user, password, cmd string) (string, error) {
	var lastErr error
	for i := 0; i < config.SSHRetries; i++ {
		configSSH := &ssh.ClientConfig{
			User:            user,
			Auth:            []ssh.AuthMethod{ssh.Password(password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         config.SSHTimeout,
		}

		addr := fmt.Sprintf("%s:%d", ip, port)
		client, err := ssh.Dial("tcp", addr, configSSH)
		if err != nil {
			lastErr = fmt.Errorf("ssh dial error: %w", err)
			time.Sleep(config.RetryBackoff)
			continue
		}

		session, err := client.NewSession()
		if err != nil {
			client.Close()
			lastErr = fmt.Errorf("ssh new session error: %w", err)
			time.Sleep(config.RetryBackoff)
			continue
		}

		output, err := session.CombinedOutput(cmd)
		session.Close()
		client.Close()

		if err != nil {
			lastErr = fmt.Errorf("ssh command error: %w", err)
			time.Sleep(config.RetryBackoff)
			continue
		}

		return string(output), nil
	}
	return "", lastErr
}

// backup

func processDevicesConcurrentlyBackup(input SSHInput, mode string, resultsChan chan<- ResultOutput) {
	concurrentDevicesSemaphore := make(chan struct{}, config.Concurrency)

	for _, device := range input.Devices {
		concurrentDevicesSemaphore <- struct{}{}

		go func(d Device) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in device processing: %v", r)
					failedResult := FailedResult{
						DeviceTypeId:  d.DeviceTypeId,
						DeviceId:      d.DeviceId,
						MetricGroupId: d.MetricGroupId,
						IP:            d.IP,
						Port:          d.Port,
						Protocol:      d.Protocol,
						Error:         fmt.Sprintf("internal-error: panic recovered: %v", r),
					}
					resultsChan <- ResultOutput{Failed: []FailedResult{failedResult}}
				}

				<-concurrentDevicesSemaphore
			}()

			var sres SuccessfulResult
			var fres *FailedResult
			sres, fres = ProcessDeviceWithTimeoutBackup(d, input.MetricIDs, mode, config.DeviceTimeout)

			if fres != nil {
				resultsChan <- ResultOutput{Failed: []FailedResult{*fres}}
			} else {
				sres.DeviceTypeId = input.DeviceTypeId
				sres.MetricGroupId = input.MetricGroupId
				resultsChan <- ResultOutput{Successful: []SuccessfulResult{sres}}
			}
		}(device)
	}

	for i := 0; i < cap(concurrentDevicesSemaphore); i++ {
		concurrentDevicesSemaphore <- struct{}{}
	}
}

func ProcessDeviceWithTimeoutBackup(device Device, metricIDs []string, mode string, timeout time.Duration) (SuccessfulResult, *FailedResult) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resChan := make(chan struct {
		s SuccessfulResult
		f *FailedResult
	}, 1)

	go func() {
		s, f := ProcessDevice(device, metricIDs, mode)
		resChan <- struct {
			s SuccessfulResult
			f *FailedResult
		}{s, f}
	}()

	select {
	case <-ctx.Done():
		return SuccessfulResult{}, &FailedResult{
			DeviceTypeId:  device.DeviceTypeId,
			DeviceId:      device.DeviceId,
			MetricGroupId: device.MetricGroupId,
			IP:            device.IP,
			Port:          device.Port,
			Protocol:      device.Protocol,
			Error:         "device-timeout",
		}
	case result := <-resChan:
		return result.s, result.f
	}
}
