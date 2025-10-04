package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"

	slonkv1 "your-org.com/slonklet/api/v1"
)

type InfoServer struct {
	sync.RWMutex

	addr            string
	slurmJobMap     map[int]*slonkv1.SlurmJob
	physicalNodeMap map[string]*slonkv1.PhysicalNode

	slurmJobsJson        []byte
	slurmJobsActiveJson  []byte
	slurmJobsRunningJson []byte
	physicalNodesJson    []byte
}

func NewInfoServer(
	addr string,
) *InfoServer {
	return &InfoServer{
		addr:            addr,
		slurmJobMap:     map[int]*slonkv1.SlurmJob{},
		physicalNodeMap: map[string]*slonkv1.PhysicalNode{},
	}
}

func (s *InfoServer) Serve() error {
	http.HandleFunc("/job/", s.handleJob)
	http.HandleFunc("/jobs", s.handleJobs)
	http.HandleFunc("/jobs/active", s.handleActiveJobs)
	http.HandleFunc("/jobs/running", s.handleRunningJobs)
	http.HandleFunc("/node/", s.handleNode)
	http.HandleFunc("/nodes", s.handleNodes)
	http.HandleFunc("/proxy/", s.handleProxy)

	log.Printf("Starting info server on %s\n", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *InfoServer) UpdateNodes(physicalNodeMap map[string]*slonkv1.PhysicalNode) error {
	s.Lock()
	defer s.Unlock()

	s.physicalNodeMap = physicalNodeMap

	physicalNodesJson, err := json.Marshal(physicalNodeMap)
	if err != nil {
		return fmt.Errorf("marshal physical nodes: %v", err)
	}
	s.physicalNodesJson = physicalNodesJson

	return nil
}

func (s *InfoServer) UpdateJobs(slurmJobMap map[int]*slonkv1.SlurmJob) error {
	s.Lock()
	defer s.Unlock()

	s.slurmJobMap = slurmJobMap

	slurmJobsJson, err := json.Marshal(slurmJobMap)
	if err != nil {
		return fmt.Errorf("marshal slurm jobs: %v", err)
	}
	s.slurmJobsJson = slurmJobsJson

	activeJobs := map[int]interface{}{}
	runningJobs := map[int]interface{}{}
	for key, value := range s.slurmJobMap {
		if !value.Status.SlurmJobRunCurrentStatus.Removed {
			activeJobs[key] = value
		}
		if value.Status.SlurmJobRunCurrentStatus.State == "RUNNING" {
			runningJobs[key] = value
		}
	}
	s.slurmJobsActiveJson, err = json.Marshal(activeJobs)
	if err != nil {
		return fmt.Errorf("marshal active slurm jobs: %v", err)
	}
	s.slurmJobsRunningJson, err = json.Marshal(runningJobs)
	if err != nil {
		return fmt.Errorf("marshal running slurm jobs: %v", err)
	}
	return nil
}

func (s *InfoServer) handleJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// URL format is /job/<jobid>
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid request, jobID is missing", http.StatusBadRequest)
		return
	}
	jobIDStr := parts[2]
	jobID, err := strconv.Atoi(jobIDStr)
	if err != nil {
		http.Error(w, "Invalid jobID format", http.StatusBadRequest)
		return
	}

	s.RLock()
	jobInfo, ok := s.slurmJobMap[jobID]
	s.RUnlock()
	if !ok {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	jsonResponse, err := json.Marshal(jobInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(jsonResponse)
}

func (s *InfoServer) handleActiveJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.Write(s.slurmJobsActiveJson)
}

func (s *InfoServer) handleJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.Write(s.slurmJobsJson)
}

func (s *InfoServer) handleRunningJobs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	w.Write(s.slurmJobsRunningJson)
}

func (s *InfoServer) handleNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// URL format is /node/<nodename>
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid request, nodeName is missing", http.StatusBadRequest)
		return
	}
	nodeName := parts[2]

	s.RLock()
	nodeInfo, ok := s.physicalNodeMap[nodeName]
	s.RUnlock()
	if !ok {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	jsonResponse, err := json.Marshal(nodeInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(jsonResponse)
}

func (s *InfoServer) handleNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.RLock()
	nodesCopy := make(map[string]interface{}, len(s.physicalNodeMap))
	for key, value := range s.physicalNodeMap {
		nodesCopy[key] = value
	}
	s.RUnlock()

	jsonResponse, err := json.Marshal(nodesCopy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(jsonResponse)
}

func (s *InfoServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	s.RLock()
	defer s.RUnlock()

	// URL format is /<node_address>/<tool>
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid request, address is missing", http.StatusBadRequest)
		return
	}
	address := parts[2]
	tool := ""
	if len(parts) >= 4 {
		tool = parts[3]
	}

	targetURL := fmt.Sprintf("http://%s/%s", address, tool)
	log.Printf("Proxying request to %s\n", targetURL)
	req, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "Error creating request", http.StatusInternalServerError)
		return
	}

	// Copy headers from the original request
	for name, values := range r.Header {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, "Error forwarding request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy headers from the response, including Content-Type
	for k, v := range resp.Header {
		if k != "Server" && k != "Date" {
			w.Header()[k] = v
		}
	}

	// Stream the response body
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}
