package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"xlsxtoSQL/config"

	"gopkg.in/yaml.v2"
)

var (
	configs     = make(map[string]*config.Config)
	configMutex sync.Mutex
)

func updateConfigHandler(w http.ResponseWriter, r *http.Request) {
	var cfg config.Config
	err := yaml.NewDecoder(r.Body).Decode(&cfg)
	if err != nil {
		http.Error(w, "Invalid YAML", 400)
		return
	}

	id := fmt.Sprintf("%d", len(configs)+1)

	configMutex.Lock()
	configs[id] = &cfg
	configMutex.Unlock()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Config %s added", id)
}

func getConfigHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	configMutex.Lock()
	cfg, exists := configs[id]
	configMutex.Unlock()
	if !exists {
		http.Error(w, "Config not found", 404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = yaml.NewEncoder(w).Encode(cfg)
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	configMutex.Lock()
	cfg, exists := configs[id]
	configMutex.Unlock()
	if !exists {
		http.Error(w, "Config not found", 404)
		return
	}

	tmpConfigFile := fmt.Sprintf("/tmp/config_%s.yaml", id)
	file, err := os.Create(tmpConfigFile)
	if err != nil {
		http.Error(w, "Failed to create temporary config file", 500)
		return
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	if err := encoder.Encode(cfg); err != nil {
		http.Error(w, "Failed to write config to file", 500)
		return
	}

	cmd := exec.Command("./main", fmt.Sprintf("--config=%s", tmpConfigFile))
	err = cmd.Start()
	if err != nil {
		log.Printf("Error starting service with config %s: %v", id, err)
		http.Error(w, "Error starting service", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Service for config %s started", id)
}

func main() {
	http.HandleFunc("/api/config", updateConfigHandler)
	http.HandleFunc("/api/get_config", getConfigHandler)
	http.HandleFunc("/api/run", runHandler)

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
