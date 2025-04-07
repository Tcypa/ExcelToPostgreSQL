package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

type Instance struct {
	ID         string    `json:"id"`
	Config     string    `json:"config"`
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	Process    *exec.Cmd `json:"-"`
	ConfigPath string    `json:"-"`
}

var (
	instances = make(map[string]*Instance)
	mutex     sync.Mutex
	counter   int
)

func extractDatabaseName(postgresURL string) string {
	parts := strings.Split(postgresURL, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func main() {
	http.HandleFunc("/api/start", withCORS(startHandler))
	http.HandleFunc("/api/stop/", withCORS(stopHandler))
	http.HandleFunc("/api/restart/", withCORS(restartHandler))
	http.HandleFunc("/api/delete/", withCORS(deleteHandler))
	http.HandleFunc("/api/instances", withCORS(instancesHandler))
	http.HandleFunc("/api/status/", withCORS(statusHandler))
	http.HandleFunc("/api/isalife/", withCORS(isalive))

	log.Println("API started on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func isalive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Server is alive")
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var req struct {
		ExcelFileName   string   `json:"excel_file_name"`
		PostgresURL     string   `json:"postgres_url"`
		IgnorantSheets  []string `json:"ignorant_sheets"`
		Once            bool     `json:"once"`
		IntervalSeconds int      `json:"interval_seconds"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	excelFilePath := fmt.Sprintf("/app/data/%s", req.ExcelFileName)

	config := map[string]interface{}{
		"excel_file_paths":     []string{excelFilePath},
		"postgres_url_base_db": req.PostgresURL,
		"interval_seconds":     req.IntervalSeconds,
		"ignorant_sheets":      req.IgnorantSheets,
	}
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		http.Error(w, "Failed to generate YAML config", http.StatusInternalServerError)
		return
	}

	displayConfig := map[string]interface{}{
		"excel_file_paths": []string{excelFilePath},
		"database_name":    extractDatabaseName(req.PostgresURL),
		"interval_seconds": req.IntervalSeconds,
		"ignorant_sheets":  req.IgnorantSheets,
	}
	displayConfigYAML, err := yaml.Marshal(displayConfig)
	if err != nil {
		http.Error(w, "Failed to generate display YAML config", http.StatusInternalServerError)
		return
	}

	counter++
	configPath := fmt.Sprintf("/tmp/config_%d.yaml", counter)
	if err := os.WriteFile(configPath, configYAML, 0644); err != nil {
		http.Error(w, "Failed to save config", http.StatusInternalServerError)
		return
	}

	cmd := exec.Command("./main", "-config", configPath)
	if req.Once {
		cmd.Args = append(cmd.Args, "-once")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to start process: %v", err), http.StatusInternalServerError)
		return
	}

	id := fmt.Sprintf("instance_%d", counter)
	mutex.Lock()
	instances[id] = &Instance{
		ID:         id,
		Config:     string(displayConfigYAML),
		Status:     "running",
		StartedAt:  time.Now(),
		Process:    cmd,
		ConfigPath: configPath,
	}
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "started"})
}

func stopHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/stop/")
	if id == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	instance, exists := instances[id]
	if !exists {
		mutex.Unlock()
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	if instance.Status == "running" {
		if err := instance.Process.Process.Kill(); err != nil {
			mutex.Unlock()
			http.Error(w, fmt.Sprintf("Failed to stop instance: %v", err), http.StatusInternalServerError)
			return
		}
		if err := instance.Process.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
				} else {
					log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
				}
			} else {
				log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
			}
		}
		instance.Status = "stopped"
	}
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "stopped"})
}

func restartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/restart/")
	if id == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	instance, exists := instances[id]
	if !exists {
		mutex.Unlock()
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	if instance.Status == "running" {
		if err := instance.Process.Process.Kill(); err != nil {
			mutex.Unlock()
			http.Error(w, fmt.Sprintf("Failed to stop instance for restart: %v", err), http.StatusInternalServerError)
			return
		}
		if err := instance.Process.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
				} else {
					log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
				}
			} else {
				log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
			}
		}
	}

	cmd := exec.Command("./main", "-config", instance.ConfigPath)
	if strings.Contains(instance.Config, "interval_seconds: 0") {
		cmd.Args = append(cmd.Args, "-once")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		mutex.Unlock()
		http.Error(w, fmt.Sprintf("Failed to restart process: %v", err), http.StatusInternalServerError)
		return
	}

	instance.Process = cmd
	instance.Status = "running"
	instance.StartedAt = time.Now()
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "restarted"})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/delete/")
	if id == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	instance, exists := instances[id]
	if !exists {
		mutex.Unlock()
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	if instance.Status == "running" {
		if err := instance.Process.Process.Kill(); err != nil {
			mutex.Unlock()
			http.Error(w, fmt.Sprintf("Failed to stop instance for deletion: %v", err), http.StatusInternalServerError)
			return
		}
		if err := instance.Process.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGKILL {
				} else {
					log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
				}
			} else {
				log.Printf("Unexpected error waiting after kill for instance %s: %v", id, err)
			}
		}
	}

	os.Remove(instance.ConfigPath)
	delete(instances, id)
	mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": "deleted"})
}

func instancesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mutex.Lock()
	defer mutex.Unlock()

	result := make([]Instance, 0, len(instances))
	for _, inst := range instances {
		if inst.Status == "running" && inst.Process.ProcessState != nil && inst.Process.ProcessState.Exited() {
			inst.Status = "stopped"
		}
		result = append(result, Instance{
			ID:        inst.ID,
			Config:    inst.Config,
			Status:    inst.Status,
			StartedAt: inst.StartedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/status/")
	if id == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	mutex.Lock()
	inst, exists := instances[id]
	mutex.Unlock()
	if !exists {
		http.Error(w, "Instance not found", http.StatusNotFound)
		return
	}

	if inst.Status == "running" && inst.Process.ProcessState != nil && inst.Process.ProcessState.Exited() {
		inst.Status = "stopped"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         inst.ID,
		"status":     inst.Status,
		"config":     inst.Config,
		"started_at": inst.StartedAt,
	})
}
