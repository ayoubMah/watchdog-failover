package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "UNKNOWN"
	}

	// print a heartbeat every second so we can see it's alive in the logs
	go func() {
		for {
			log.Printf("[%s] I am alive!", instanceID)
			time.Sleep(1 * time.Second)
		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("PING received on %s", instanceID)
		fmt.Fprintf(w, "I am alive! ID: %s", instanceID)
	})

	log.Printf("Starting victim app [%s] on :9995", instanceID)
	log.Fatal(http.ListenAndServe(":9995", nil))
}
