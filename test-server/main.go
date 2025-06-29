package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type GameEvent struct {
	EventType            string `json:"eventType"`
	StartTime            string `json:"startTime"`
	EndTime              string `json:"endTime"`
	TargetMicroservice   string `json:"targetMicroservice"`
}

func main() {
	http.HandleFunc("/api/events", handleEvents)
	http.HandleFunc("/health", handleHealth)
	
	fmt.Println("Starting test game event API server on :8080")
	fmt.Println("Available endpoints:")
	fmt.Println("  GET /api/events - Returns game events")
	fmt.Println("  GET /health - Health check")
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create events with current time
	now := time.Now()
	
	events := []GameEvent{
		{
			EventType:          "MassPvPEvent",
			StartTime:          now.Add(-45 * time.Minute).Format(time.RFC3339),
			EndTime:            now.Add(-15 * time.Minute).Format(time.RFC3339),
			TargetMicroservice: "pvp-battle-service",
		},
		{
			EventType:          "RaidBossSpawn",
			StartTime:          now.Add(1 * time.Minute).Format(time.RFC3339),
			EndTime:            now.Add(3 * time.Minute).Format(time.RFC3339),
			TargetMicroservice: "raid-instance-manager",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	
	json.NewEncoder(w).Encode(events)
	
	fmt.Printf("[%s] GET /api/events - Returned %d events\n", time.Now().Format("2006-01-02 15:04:05"), len(events))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
} 