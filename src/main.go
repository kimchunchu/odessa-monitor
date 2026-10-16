package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

type MonitoringStatus struct {
	Healthy              bool   `json:"healthy"`
	ConsecutiveFailures  int    `json:"consecutive_failures"`
	ResponseTime         string `json:"response_time"`
	MemoryTotalBytes     uint64 `json:"memory_total_bytes"`
	MemoryAvailableBytes uint64 `json:"memory_available_bytes"`
	DiskTotalBytes       uint64 `json:"disk_total_bytes"`
	DiskAvailableBytes   uint64 `json:"disk_available_bytes"`
}

var (
	statusMutex   sync.RWMutex
	currentStatus = MonitoringStatus{
		Healthy:      false,
		ResponseTime: "not checked",
	}
)

func readDisk() (uint64, uint64, error) {
	var stat unix.Statfs_t

	if err := unix.Statfs("/", &stat); err != nil {
		return 0, 0, err
	}

	total := stat.Blocks * uint64(stat.Bsize)
	available := stat.Bavail * uint64(stat.Bsize)

	return total, available, nil
}

func readMemory() (uint64, uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")

	if err != nil {
		return 0, 0, err
	}

	var total uint64
	var available uint64

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)

		if err != nil {
			return 0, 0, err
		}

		switch fields[0] {
		case "MemTotal:":
			total = value
		case "MemAvailable:":
			available = value
		}
	}

	if total == 0 || available == 0 {
		return 0, 0, fmt.Errorf("memory information not found")
	}

	return total * 1024, available * 1024, nil
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	statusMutex.RLock()
	status := currentStatus
	statusMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func checkHealth(client *http.Client, url string) (bool, string) {
	start := time.Now()
	resp, err := client.Get(url)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("DOWN: %v", err)
		return false, elapsed.String()
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("UNHEALTHY: HTTP %d", resp.StatusCode)
		return false, elapsed.String()
	}

	log.Printf("UP: status=%d elapsed=%dms", resp.StatusCode, elapsed)
	return true, elapsed.String()
}

func main() {
	http.HandleFunc("/status", statusHandler)

	go func() {
		log.Println("Monitor API listening on :8081")
		log.Fatal(http.ListenAndServe(":8081", nil))
	}()

	consecutiveFailures := 0

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	url := "http://127.0.0.1/health/live"
	ticker := time.NewTicker(10 * time.Second)

	defer ticker.Stop()

	for {
		healthy, response_time := checkHealth(client, url)
		if healthy {
			consecutiveFailures = 0
		} else {
			consecutiveFailures++
			if consecutiveFailures == 3 {
				log.Println("ALERT: 3 consecutive failures")
			}
		}

		memoryTotal, memoryAvailable, err := readMemory()

		if err != nil {
			log.Printf("MEMORY ERROR: %v", err)
		}

		diskTotal, diskAvailable, err := readDisk()

		if err != nil {
			log.Printf("DISK ERROR: %v", err)
		}

		statusMutex.Lock()
		currentStatus = MonitoringStatus{
			Healthy:              healthy,
			ConsecutiveFailures:  consecutiveFailures,
			ResponseTime:         response_time,
			MemoryTotalBytes:     memoryTotal,
			MemoryAvailableBytes: memoryAvailable,
			DiskTotalBytes:       diskTotal,
			DiskAvailableBytes:   diskAvailable,
		}
		statusMutex.Unlock()

		<-ticker.C
	}
}
