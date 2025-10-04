package main

import (
	"encoding/json"
	"log"
	"net"
	"os"

	"your-org.com/slonklet/internal/slurm"
)

func main() {
	socketPath := "/etc/slurm/slurmrestd/slurmrestd.sock"
	data := []slurm.SlurmNode{
		{
			Name: "slurm-node-1",
			// State:    []string{"ALLOCATED", "RESERVED"},
			State:    []string{"MIXED"},
			Features: []string{"h100", "gpu"},
			Reason:   "test-reason-1",
			Comment:  "PhysicalHost:/abc/edf/abc",
		},
		{
			Name:     "slurm-node-2",
			State:    []string{"MIXED"},
			Features: []string{"h100", "gpu"},
			Reason:   "test-reason-2",
			Comment:  "PhysicalHost:/123/456/789",
		},
	}

	if err := os.RemoveAll(socketPath); err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		json.NewEncoder(conn).Encode(data)
		conn.Close()
	}
}
