package slurm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	// Replace with the path to your CRD API
)

func TestFetchSlurmNodes(t *testing.T) {
	// Setup test data.
	testData := SlurmResponse{
		Nodes: []SlurmNode{
			{
				Name:     "slurm-node-1",
				State:    []string{"ALLOCATED", "RESERVED"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-1",
			},
		},
	}

	// Start the test server.
	socketPath := "/tmp/test.sock"
	cleanup, err := StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer cleanup()

	// Fetch the test data once.
	nodes, err := ListSlurmNodes(socketPath)
	assert.NoError(t, err)
	assert.Equal(t, "slurm-node-1", nodes[0].Name)
	assert.Equal(t, []string{"ALLOCATED", "RESERVED"}, nodes[0].State)
}

func TestParsePhysicalIDfromComment(t *testing.T) {
	// Test single comment.
	comment := "PhysicalHost:/abc/edf/abc,test:test"
	physicalID, err := parsePhysicalHostNameFromComment(comment)
	assert.NoError(t, err)
	assert.Equal(t, "abc", physicalID)

	// Test multiple comments.
	comment = "PhysicalHost:/abc/edf/abc,PhysicalHost:/123/456/789"
	physicalID, err = parsePhysicalHostNameFromComment(comment)
	assert.Error(t, err)
	assert.Equal(t, "", physicalID)
}

func TestSyncSlurmJobs(t *testing.T) {
	// Get the current Unix timestamp.
	now := time.Now().Unix()
	// Setup test data.
	testData := SlurmResponse{
		Jobs: []SlurmJob{
			{
				JobID: 1,
				Name:  "test-job-1",
				SubmitTime: FlagType{
					Number:   int(now),
					Set:      true,
					Infinite: false,
				},
				Container:     "container-1",
				Cluster:       "cluster-1",
				TimeMinimum:   FlagType{Number: 1, Set: true, Infinite: false},
				MemoryPerTRES: "1G",
				JobState:      "RUNNING",
			},
			{ //job submitted 8 days ago
				JobID: 2,
				Name:  "test-job-2",
				SubmitTime: FlagType{
					Number:   int(now - 8*24*60*60), // 8 days ago
					Set:      true,
					Infinite: false,
				},
				Container:     "container-2",
				Cluster:       "cluster-2",
				TimeMinimum:   FlagType{Number: 2, Set: true, Infinite: false},
				MemoryPerTRES: "2G",
				JobState:      "PENDING",
			},
			{ //job submitted 8 days ago, but last run 6 days ago
				JobID: 3,
				Name:  "test-job-3",
				SubmitTime: FlagType{
					Number:   int(now - 8*24*60*60), // 8 days ago
					Set:      true,
					Infinite: false,
				},
				StartTime: FlagType{
					Number:   int(now - 6*24*60*60), // 6 days ago
					Set:      true,
					Infinite: false,
				},
				Container:     "container-3",
				Cluster:       "cluster-3",
				TimeMinimum:   FlagType{Number: 1, Set: true, Infinite: false},
				MemoryPerTRES: "1G",
				JobState:      "PENDING",
			},
		},
	}

	// Start the test server.
	socketPath := "/tmp/test.sock"
	cleanup, err := StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer cleanup()

	// Fetch all jobs, should be 3 of them
	jobs, err := SyncSlurmJobs(socketPath)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(jobs))
	assert.Equal(t, 1, jobs[0].JobID)
	assert.Equal(t, "test-job-1", jobs[0].Name)
	assert.Equal(t, "container-1", jobs[0].Container)
	assert.Equal(t, "cluster-1", jobs[0].Cluster)
	assert.Equal(t, "1G", jobs[0].MemoryPerTRES)
	assert.Equal(t, "RUNNING", jobs[0].JobState)
	assert.Equal(t, 2, jobs[1].JobID)
	assert.Equal(t, "test-job-2", jobs[1].Name)
	assert.Equal(t, "container-2", jobs[1].Container)
	assert.Equal(t, "cluster-2", jobs[1].Cluster)
	assert.Equal(t, "2G", jobs[1].MemoryPerTRES)
	assert.Equal(t, "PENDING", jobs[1].JobState)
	assert.Equal(t, 3, jobs[2].JobID)
	assert.Equal(t, "test-job-3", jobs[2].Name)
	assert.Equal(t, "container-3", jobs[2].Container)
	assert.Equal(t, "cluster-3", jobs[2].Cluster)
	assert.Equal(t, "1G", jobs[2].MemoryPerTRES)
	assert.Equal(t, "PENDING", jobs[1].JobState)

	states := []string{"PENDING"}

	// Getting only pending jobs that are longer than 7 days, have less than 64 nodes and are not the hiro partition
	pendingJobs, err := listJobsByStateAndAge(socketPath, 7, states, 64, "hero")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(pendingJobs))
	assert.Equal(t, 2, pendingJobs[0].JobID)
	assert.Equal(t, "test-job-2", pendingJobs[0].Name)
	assert.Equal(t, "container-2", pendingJobs[0].Container)
	assert.Equal(t, "cluster-2", pendingJobs[0].Cluster)
	assert.Equal(t, "2G", pendingJobs[0].MemoryPerTRES)
	assert.Equal(t, "PENDING", pendingJobs[0].JobState)

	//Cancel the pending job, now there should be 2 remaining (jobs 1 and 3)
	err = cancelHeldJobsLongerThanAWeek(socketPath, false)
	assert.NoError(t, err)
	cleaned_jobs, err := SyncSlurmJobs(socketPath)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(cleaned_jobs))
	assert.Equal(t, 1, cleaned_jobs[0].JobID)
	assert.Equal(t, "test-job-1", cleaned_jobs[0].Name)
	assert.Equal(t, "container-1", cleaned_jobs[0].Container)
	assert.Equal(t, "cluster-1", cleaned_jobs[0].Cluster)
	assert.Equal(t, "1G", cleaned_jobs[0].MemoryPerTRES)
	assert.Equal(t, "RUNNING", cleaned_jobs[0].JobState)
	assert.Equal(t, 3, cleaned_jobs[1].JobID)
	assert.Equal(t, "test-job-3", cleaned_jobs[1].Name)
	assert.Equal(t, "container-3", cleaned_jobs[1].Container)
	assert.Equal(t, "cluster-3", cleaned_jobs[1].Cluster)
	assert.Equal(t, "1G", cleaned_jobs[1].MemoryPerTRES)
	assert.Equal(t, "PENDING", cleaned_jobs[1].JobState)

	// Fetch the pending jobs, should be none now that we killed it
	pendingJobsNew, err := listJobsByStateAndAge(socketPath, 7, states, 64, "hero")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(pendingJobsNew))
}

func TestParseJobNodeList(t *testing.T) {
	nodes, err := ParseJobNodeList("cluster-h100-0-0")
	assert.NoError(t, err)
	assert.Equal(t, []string{"cluster-h100-0-0"}, nodes)

	nodes, err = ParseJobNodeList("cluster-h100-0-0,cluster-h100-1-1,cluster-h100-2-2")
	assert.NoError(t, err)
	assert.Equal(t, []string{"cluster-h100-0-0", "cluster-h100-1-1", "cluster-h100-2-2"}, nodes)

	nodes, err = ParseJobNodeList("cluster-h100-0-[0-2,4,6-8]")
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"cluster-h100-0-0",
		"cluster-h100-0-1",
		"cluster-h100-0-2",
		"cluster-h100-0-4",
		"cluster-h100-0-6",
		"cluster-h100-0-7",
		"cluster-h100-0-8",
	}, nodes)

	nodes, err = ParseJobNodeList("cluster-h100-0-[0-2,18-20,25-27],cluster-h100-1-[0-1,3-4,6],cluster-h100-2-[0-2,41-43,74,76]")
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"cluster-h100-0-0",
		"cluster-h100-0-1",
		"cluster-h100-0-2",
		"cluster-h100-0-18",
		"cluster-h100-0-19",
		"cluster-h100-0-20",
		"cluster-h100-0-25",
		"cluster-h100-0-26",
		"cluster-h100-0-27",
		"cluster-h100-1-0",
		"cluster-h100-1-1",
		"cluster-h100-1-3",
		"cluster-h100-1-4",
		"cluster-h100-1-6",
		"cluster-h100-2-0",
		"cluster-h100-2-1",
		"cluster-h100-2-2",
		"cluster-h100-2-41",
		"cluster-h100-2-42",
		"cluster-h100-2-43",
		"cluster-h100-2-74",
		"cluster-h100-2-76",
	},
		nodes,
	)

}
