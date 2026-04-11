package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"
)

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		log.Fatalf("invalid URL %q: %v", raw, err)
	}
	return u
}

func main() {
	primaryURL := envOrDefault("PRIMARY_URL", "http://victim-a:9995/status")
	backupURL := envOrDefault("BACKUP_URL", "http://victim-b:9995/status")
	primaryBackend := envOrDefault("PRIMARY_BACKEND", "http://victim-a:9995")
	backupBackend := envOrDefault("BACKUP_BACKEND", "http://victim-b:9995")
	primaryContainer := envOrDefault("PRIMARY_CONTAINER", "victim-a")
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

	// cancel on SIGTERM or Ctrl-C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var backupStarted atomic.Bool
	failureCount := 0
	client := &http.Client{Timeout: 2 * time.Second}

	// reverse proxy — routes to primary or backup based on current state
	primaryTarget := mustParseURL(primaryBackend)
	backupTarget := mustParseURL(backupBackend)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			var target *url.URL
			if backupStarted.Load() {
				target = backupTarget
			} else {
				target = primaryTarget
			}
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	mux := http.NewServeMux()

	// :9999 — watchdog status
	statusMux := http.NewServeMux()
	statusMux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if backupStarted.Load() {
			fmt.Fprintln(w, "watchdog: victim-a is DOWN, victim-b is running")
		} else {
			fmt.Fprintln(w, "watchdog: victim-a is UP")
		}
	})

	// :9998 — transparent proxy to active backend
	mux.Handle("/", proxy)

	statusServer := &http.Server{Addr: ":9999", Handler: statusMux}
	proxyServer := &http.Server{Addr: ":9998", Handler: mux}

	go func() {
		if err := statusServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("status server error: %v", err)
		}
	}()
	go func() {
		log.Printf("Proxy listening on :9998 → routing to active backend")
		if err := proxyServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("proxy server error: %v", err)
		}
	}()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			if backupStarted.Load() {
				// poll backup liveness
				resp, err := client.Get(backupURL)
				if err != nil {
					log.Printf("[DOWN] victim-b is unreachable: %v", err)
				} else {
					resp.Body.Close()
					log.Printf("[ALIVE] victim-b responded: %s", resp.Status)
				}

				// check if primary recovered
				resp, err = client.Get(primaryURL)
				if err == nil {
					resp.Body.Close()
					log.Printf("[RECOVERED] %s is back online — switching proxy back to primary", primaryContainer)
					backupStarted.Store(false)
					failureCount = 0
				}
				continue
			}

			// normal primary health check
			resp, err := client.Get(primaryURL)
			if err != nil {
				failureCount++
				log.Printf("[DOWN] victim-a is unreachable: %v (failure %d/%d)", err, failureCount, failureThreshold)

				if failureCount >= failureThreshold {
					// start backup
					log.Printf("[ACTION] Starting backup container: %s", backupContainer)
					cmd := exec.Command("docker", "start", backupContainer)
					out, err := cmd.CombinedOutput()
					if err != nil {
						log.Printf("[ERROR] Failed to start %s: %v\n%s", backupContainer, err, out)
					} else {
						log.Printf("[OK] %s started — proxy now routes to backup", backupContainer, out)
						backupStarted.Store(true)
					}

					// attempt to restart primary so it can recover
					log.Printf("[ACTION] Attempting to restart primary: %s", primaryContainer)
					cmd = exec.Command("docker", "start", primaryContainer)
					out, err = cmd.CombinedOutput()
					if err != nil {
						log.Printf("[WARN] Could not restart %s: %v\n%s", primaryContainer, err, out)
					} else {
						log.Printf("[OK] %s restart initiated: %s", primaryContainer, out)
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

	log.Println("Shutting down watchdog...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := statusServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("status server shutdown error: %v", err)
	}
	if err := proxyServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("proxy server shutdown error: %v", err)
	}
	log.Println("Watchdog stopped.")
}
