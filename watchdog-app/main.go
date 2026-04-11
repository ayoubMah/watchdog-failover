package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	primaryURL := envOrDefault("PRIMARY_URL", "http://victim-a:9995/status")
	backupURL := envOrDefault("BACKUP_URL", "http://victim-b:9995/status")
	backupContainer := envOrDefault("BACKUP_CONTAINER", "victim-b")

	checkInterval, err := time.ParseDuration(envOrDefault("CHECK_INTERVAL", "5s"))
	if err != nil {
		log.Fatalf("invalid CHECK_INTERVAL: %v", err)
	}

	failureThreshold, err := strconv.Atoi(envOrDefault("FAILURE_THRESHOLD", "3"))
	if err != nil || failureThreshold < 1 {
		log.Fatalf("invalid FAILURE_THRESHOLD: must be a positive integer")
	}

	log.Printf("Watchdog started. primary=%s backup=%s interval=%s threshold=%d",
		primaryURL, backupURL, checkInterval, failureThreshold)

	var backupStarted atomic.Bool
	failureCount := 0
	client := &http.Client{Timeout: 2 * time.Second}

	// expose a simple status endpoint for the watchdog itself
	go func() {
		http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			if backupStarted.Load() {
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
		if backupStarted.Load() {
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
			failureCount++
			log.Printf("[DOWN] victim-a is unreachable: %v (failure %d/%d)", err, failureCount, failureThreshold)

			if failureCount >= failureThreshold {
				log.Printf("[ACTION] Starting backup container: %s", backupContainer)
				cmd := exec.Command("docker", "start", backupContainer)
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Printf("[ERROR] Failed to start %s: %v\n%s", backupContainer, err, out)
				} else {
					log.Printf("[OK] %s started successfully: %s", backupContainer, out)
					backupStarted.Store(true)
				}
			}
		} else {
			if failureCount > 0 {
				log.Printf("[RECOVERED] victim-a is back after %d failure(s), resetting counter", failureCount)
				failureCount = 0
			}
			resp.Body.Close()
			log.Printf("[ALIVE] victim-a responded: %s", resp.Status)
		}
	}
}
