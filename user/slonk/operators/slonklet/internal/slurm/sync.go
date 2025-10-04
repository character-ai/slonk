package slurm

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

func ListSlurmNodes(socketPath string) ([]SlurmNode, error) {
	if socketPath == "" {
		return listSlurmNodesFromCommand()
	}
	return listSlurmNodes(socketPath)
}

func listSlurmNodesFromCommand() ([]SlurmNode, error) {
	resp, err := exec.Command("scontrol", "show", "node", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("running scontrol command: %s", err)
	}

	var slurmResponse SlurmResponse
	if err := json.Unmarshal(resp, &slurmResponse); err != nil {
		return nil, fmt.Errorf("decode slurm node list: %s", err)
	}

	return slurmResponse.Nodes, nil
}

func listSlurmNodes(socketPath string) ([]SlurmNode, error) {
	// Communicate with local socket file.
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("http://localhost:8080/slurm/v0.0.40/nodes")
	if err != nil {
		return nil, fmt.Errorf("making request:: %s", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %s", err)
	}

	var slurmResponse SlurmResponse
	if err := json.Unmarshal(data, &slurmResponse); err != nil {
		return nil, fmt.Errorf("decode slurm node list: %s", err)
	}

	return slurmResponse.Nodes, nil
}

func RestartSlurmRestD() error {
	// Restart slurmrestd.
	if err := exec.Command("service", "slurmrestd", "restart").Run(); err != nil {
		return fmt.Errorf("restart slurmrestd: %s", err)
	}
	return nil
}

func parsePhysicalHostNameFromComment(comment string) (string, error) {
	if comment == "" {
		return "", fmt.Errorf("empty comment")
	}

	var result string
	parts := strings.Split(comment, ",")
	for _, part := range parts {
		// TODO (yiran): Regex validate the hostname.
		if strings.HasPrefix(part, "PhysicalHost:") {
			if result != "" {
				return "", fmt.Errorf("multiple PhysicalHost comments")
			}
			result = strings.TrimPrefix(part, "PhysicalHost:")
			if result == "" {
				return "", fmt.Errorf("empty PhysicalHost value")
			}
			result = path.Base(result)
			if result == "" {
				return "", fmt.Errorf("empty PhysicalHost value")
			}
		}
	}

	return result, nil
}

func SyncSlurmJobs(socketPath string) ([]SlurmJob, error) {
	if socketPath == "" {
		return syncSlurmJobsFromCommand()
	}
	return syncSlurmJobsFromSocket(socketPath)
}

func syncSlurmJobsFromCommand() ([]SlurmJob, error) {
	resp, err := exec.Command("squeue", "-a", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("running squeue command: %s", err)
	}

	var slurmResponse SlurmResponse
	if err := json.Unmarshal(resp, &slurmResponse); err != nil {
		return nil, fmt.Errorf("decode slurm job list: %s", err)
	}

	return slurmResponse.Jobs, nil
}

// SyncSlurmJobs fetches the list of jobs from the slurmrestd server.
func syncSlurmJobsFromSocket(socketPath string) ([]SlurmJob, error) {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://localhost:8080/slurm/v0.0.40/jobs")
	if err != nil {
		return nil, fmt.Errorf("making request: %s", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %s", err)
	}

	var jobsResponse SlurmResponse
	if err := json.Unmarshal(data, &jobsResponse); err != nil {
		return nil, fmt.Errorf("decoding slurm job list: %s", err)
	}
	fmt.Println("Fetching all jobs: ")
	fmt.Printf("%-8s %-19s %-10s %-19s %-19s %-19s %-10s %s\n", "Job ID", "Submit Time", "Job State", "Start Time", "End Time", "Preempt Time", "Node Count", "Partition")
	for _, job := range jobsResponse.Jobs {
		fmt.Printf("%-8d %-19s %-10s %-19s %-19s %-19s %-10d %s\n", job.JobID, time.Unix(int64(job.SubmitTime.Number), 0).Format("2006-01-02 15:04:05"), job.JobState, time.Unix(int64(job.StartTime.Number), 0).Format("2006-01-02 15:04:05"), time.Unix(int64(job.EndTime.Number), 0).Format("2006-01-02 15:04:05"), time.Unix(int64(job.PreemptTime.Number), 0).Format("2006-01-02 15:04:05"), job.NodeCount.Number, job.Partition)
	}

	return jobsResponse.Jobs, nil
}

func listJobsByStateAndAge(socketPath string, expirationDays int, states []string, lessThanNumNodes int, excludePartition string) ([]SlurmJob, error) {
	allJobs, err := SyncSlurmJobs(socketPath)
	if err != nil {
		return nil, err
	}

	var heldJobs []SlurmJob
	expirationTime := time.Now().AddDate(0, 0, -expirationDays).Unix()
	// fmt.Printf("Expiration time: %d\n", expirationTime)

	// Convert states slice to map for efficient lookup
	stateMap := make(map[string]bool)
	for _, state := range states {
		stateMap[state] = true
	}
	for _, job := range allJobs {
		lastTouchedTime := job.SubmitTime.Number
		if job.StartTime.Set && job.StartTime.Number > lastTouchedTime {
			lastTouchedTime = job.StartTime.Number
		}
		if job.EndTime.Set && job.EndTime.Number > lastTouchedTime {
			lastTouchedTime = job.EndTime.Number
		}

		// fmt.Printf("Job Time %d\n", job.SubmitTime)
		// Checking if SubmitTime is true, and the job was submitted before the expiration time
		if int64(lastTouchedTime) < expirationTime {
			state := job.JobState
			if stateMap[state] && job.NodeCount.Number < lessThanNumNodes && job.Partition != excludePartition {
				heldJobs = append(heldJobs, job)
			}
		}
	}

	return heldJobs, nil
}

func ParseJobNodeList(input string) ([]string, error) {
	if input == "" {
		return []string{}, nil
	}

	var result []string
	// Initialize the index for iteration and slices to store parts.
	start := 0
	parts := []string{}

	// Iterate over the string to manually split into parts.
	for i := 0; i < len(input); i++ {
		if input[i] == ',' && (i+1 < len(input) && input[i+1] != ' ') {
			isRange := false
			// Check if we're inside brackets.
			for j := i; j >= 0; j-- {
				if input[j] == '[' {
					isRange = true
					break
				} else if input[j] == ']' {
					break
				}
			}
			if !isRange {
				parts = append(parts, input[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, input[start:])

	fmt.Printf("Parts: %v\n", parts)

	// Process each part for base and ranges.
	for _, part := range parts {
		index := strings.Index(part, "-[")
		if index == -1 {
			result = append(result, part)
			continue
		}

		base := part[:index] + "-"
		rangesPart := strings.Trim(part[index+2:], "[]")
		ranges := strings.Split(rangesPart, ",")

		for _, r := range ranges {
			if strings.Contains(r, "-") {
				bounds := strings.Split(r, "-")
				start, err := strconv.Atoi(bounds[0])
				if err != nil {
					return nil, err
				}
				end, err := strconv.Atoi(bounds[1])
				if err != nil {
					return nil, err
				}
				for i := start; i <= end; i++ {
					result = append(result, fmt.Sprintf("%s%d", base, i))
				}
			} else {
				result = append(result, base+r)
			}
		}
	}
	return result, nil
}

func cancelSlurmJob(socketPath string, jobID int) error {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", socketPath)
			},
		},
		Timeout: 10 * time.Second,
	}

	url := fmt.Sprintf("http://localhost:8080/slurm/v0.0.40/job/%d", jobID)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("creating request to kill job: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request to kill job: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to kill job %d, status code: %d", jobID, resp.StatusCode)
	}

	return nil
}

func cancelHeldJobsLongerThanAWeek(socketPath string, dryRun bool) error {
	// list jobs from earlier than 7 days ago that are any of the states below
	// PD PENDING Job is awaiting resource allocation

	jobsToBeCancelled, err := listJobsByStateAndAge(socketPath, 7, []string{"PENDING"}, 64, "hero")
	if err != nil {
		return err
	}

	if len(jobsToBeCancelled) == 0 {
		fmt.Println("No held jobs found.")
	}

	for _, job := range jobsToBeCancelled {
		if dryRun {
			fmt.Printf("Dry run: would have requested kill for job %d\n", job.JobID)
		} else {
			err := cancelSlurmJob(socketPath, job.JobID)
			if err != nil {
				fmt.Printf("Error killing job %d: %v\n", job.JobID, err)
			} else {
				fmt.Printf("Successfully requested kill for job %d\n", job.JobID)
			}
		}
	}

	return nil
}
