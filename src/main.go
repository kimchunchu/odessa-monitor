package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type MonitoringStatus struct {
	Healthy             bool   `json:"healthy"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	ResponseTime        string `json:"response_time"`
}

var (
	statusMutex   sync.RWMutex
	currentStatus = MonitoringStatus{
		Healthy:      false,
		ResponseTime: "not checked",
	}
)

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

		statusMutex.Lock()
		currentStatus = MonitoringStatus{
			Healthy:             healthy,
			ConsecutiveFailures: consecutiveFailures,
			ResponseTime:        response_time,
		}
		statusMutex.Unlock()

		<-ticker.C
	}
}
