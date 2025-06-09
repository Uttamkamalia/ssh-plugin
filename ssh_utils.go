package main

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"golang.org/x/crypto/ssh"
)

var MetricCommandMap = map[string]string{
	"cpu_usage":    `top -bn1 | grep "Cpu(s)" | awk '{print $2 + $4}'`,
	"memory_usage": `free | awk '/Mem:/ { printf("%.2f", $3/$2 * 100.0) }'`,
	"disk_usage":   `df / | awk 'NR==2 { print $5 }'`,
	"uptime":       `uptime -p`,
	"uname":        `uname -a`,
}

func ProcessDeviceWithTimeout(device Device, metricIDs []string, mode string, timeout time.Duration) (SuccessfulResult, *FailedResult) {
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

func ProcessDevice(device Device, metricIDs []string, mode string) (SuccessfulResult, *FailedResult) {
	if mode == "DISCOVERY" {
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
