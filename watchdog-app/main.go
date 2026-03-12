package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"
)

const (
	primaryURL      = "http://victim-a:9995/status"
	backupURL       = "http://victim-b:9995/status"
	backupContainer = "victim-b"
	checkInterval   = 5 * time.Second
)

func main() {
	backupStarted := false
	client := &http.Client{Timeout: 2 * time.Second}

	log.Println("Watchdog started. Monitoring victim-a every 5s...")

	// expose a simple status endpoint for the watchdog itself
	go func() {
		http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			if backupStarted {
				fmt.Fprintln(w, "watchdog: victim-a is DOWN, victim-b is running")
			} else {
				fmt.Fprintln(w, "watchdog: victim-a is UP")
			}
		})
		log.Fatal(http.ListenAndServe(":9999", nil))
	}()

	ticker := time.NewTicker(checkInterval)
	for range ticker.C {
		// after failover: poll victim-b and keep logging its liveness
		if backupStarted {
			resp, err := client.Get(backupURL)
			if err != nil {
				log.Printf("[DOWN] victim-b is unreachable: %v", err)
			} else {
				resp.Body.Close()
				log.Printf("[ALIVE] victim-b responded: %s", resp.Status)
			}
			continue
		}

		resp, err := client.Get(primaryURL)
		if err != nil {
			log.Printf("[DOWN] victim-a is unreachable: %v", err)
			log.Printf("[ACTION] Starting backup container: %s", backupContainer)

			cmd := exec.Command("docker", "start", backupContainer)
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("[ERROR] Failed to start %s: %v\n%s", backupContainer, err, out)
			} else {
				log.Printf("[OK] %s started successfully: %s", backupContainer, out)
				backupStarted = true
			}
		} else {
			resp.Body.Close()
			log.Printf("[ALIVE] victim-a responded: %s", resp.Status)
		}
	}
}
