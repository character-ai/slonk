package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"your-org.com/slonklet/internal/task/core"
	"your-org.com/slonklet/internal/task/queue"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	flag.Parse()

	localQueue, err := queue.NewLocalQueue("/etc/slurm/slonklet/task_state", "/etc/slurm/slonklet/task_log", log.Default())
	if err != nil {
		log.Fatalf("Failed to create local queue: %s\n", err)
	}

	http.HandleFunc("/job/enqueue/command", func(w http.ResponseWriter, r *http.Request) {
		taskID := core.TaskID(uuid.New())
		command := r.URL.Query().Get("command")
		if command == "" {
			http.Error(w, "command is required", http.StatusBadRequest)
		}
		localQueue.EnqueueShellCommandTask(taskID, command)
	})
	http.HandleFunc("/job/state", func(w http.ResponseWriter, r *http.Request) {
		taskIDStr := r.URL.Query().Get("id")
		taskID, err := uuid.Parse(taskIDStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}

		task, err := localQueue.GetTask(core.TaskID(taskID))
		if err != nil {
			if err == os.ErrNotExist {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Write([]byte(task.State))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting server on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
