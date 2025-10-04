package slurm

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

// StartTestServer starts a Unix socket server for testing
func StartTestSlurmRestD(socketPath string, response SlurmResponse) (func(), error) {
	if err := os.RemoveAll(socketPath); err != nil {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			// Extract jobID from the request URL.
			jobID := strings.TrimPrefix(r.URL.Path, "/slurm/v0.0.40/job/")
			jobIDInt, err := strconv.Atoi(jobID)
			if err != nil {
				// handle error
			}
			// Create a new slice to hold the jobs that weren't deleted.
			remainingJobs := make([]SlurmJob, 0)
			for _, job := range response.Jobs {
				if job.JobID != jobIDInt {
					remainingJobs = append(remainingJobs, job)
				}
			}
			// Replace the old Jobs slice with the new one.
			response.Jobs = remainingJobs
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// For all other methods, return the current state of the testData.
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	server := &http.Server{
		Handler: mux,
	}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	return func() {
		if err := server.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown the server: %v", err)
		}
		os.RemoveAll(socketPath)
	}, nil
}
