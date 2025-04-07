package test

import (
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8080"

func Test_waitForServer(t *testing.T) {
	timeout := time.After(10 * time.Second)
	tick := time.Tick(500 * time.Millisecond)

	for {
		select {
		case <-timeout:
			t.Fatal("Server not ready in time")
		case <-tick:
			resp, err := http.Get(baseURL + "/api/isalive")
			if err == nil && resp.StatusCode == 200 {
				return
			}
		}
	}
}
