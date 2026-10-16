package main

import (
	"net/http"
	"time"
	"log"
)

func checkHealth(client *http.Client, url string) {
	resp, err := client.Get(url)

	if err != nil {
		log.Printf("DOWN: %v", err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("UNHEALTHY: HTTP %d", resp.StatusCode)
		return
	}

	log.Printf("UP: HTTP %d", resp.StatusCode)
}

func main() {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	url := "http://127.0.0.1/health/live"
	ticker := time.NewTicker(10 * time.Second)

	defer ticker.Stop()

	for {
		checkHealth(client, url)
		<-ticker.C
	}
}
