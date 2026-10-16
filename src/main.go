package main

import (
	"net/http"
	"time"
	"log"
)

func checkHealth(client *http.Client, url string) bool {
	start := time.Now();
	resp, err := client.Get(url)
	elapsed := time.Since(start);

	if err != nil {
		log.Printf("DOWN: %v", err)
		return false;
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("UNHEALTHY: HTTP %d", resp.StatusCode)
		return false;
	}

	log.Printf("UP: status=%d elapsed=%dms", resp.StatusCode, elapsed)
	return true;
}

func main() {
	consecutiveFailures := 0

	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	url := "http://127.0.0.1/health/live"
	ticker := time.NewTicker(10 * time.Second)

	defer ticker.Stop()

	for {
		healthy := checkHealth(client, url)
		if healthy {
			consecutiveFailures = 0;
		} else {
			consecutiveFailures++;
			if consecutiveFailures == 3 {
				log.Println("ALERT: 3 consecutive failures")
			}
		}
		<-ticker.C
	}
}
