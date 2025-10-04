package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlurmJobRunStatusIsEqual(t *testing.T) {
	s1 := SlurmJob{
		Spec: SlurmJobSpec{
			Command: "echo hello",
		},
		Status: SlurmJobStatus{
			RestartCount: 1,
			SlurmJobRunCurrentStatus: SlurmJobRunStatus{
				RunID:   1,
				Removed: false,
				State:   "RUNNING",
			},
		},
	}
	s2 := SlurmJob{
		Spec: SlurmJobSpec{
			Command: "echo hello",
		},
		Status: SlurmJobStatus{
			RestartCount: 1,
			SlurmJobRunCurrentStatus: SlurmJobRunStatus{
				RunID:   1,
				Removed: false,
				State:   "RUNNING",
			},
		},
	}
	assert.True(t, s1.Status.SlurmJobRunCurrentStatus.IsEqual(s2.Status.SlurmJobRunCurrentStatus))

	s1.Status.SlurmJobRunCurrentStatus.PhysicalNodeSnapshots = map[string]*PhysicalNodeSnapshot{
		"node2": {
			PhysicalNodeName: "node2",
			K8sNodeName:      "pnode2",
			SlurmNodeName:    "snode2",
		},
		"node1": {
			PhysicalNodeName: "node1",
			K8sNodeName:      "pnode1",
			SlurmNodeName:    "snode1",
		},
	}
	s2.Status.SlurmJobRunCurrentStatus.PhysicalNodeSnapshots = map[string]*PhysicalNodeSnapshot{
		"node1": {
			PhysicalNodeName: "node1",
			K8sNodeName:      "pnode1",
			SlurmNodeName:    "snode1",
		},
		"node2": {
			PhysicalNodeName: "node2",
			K8sNodeName:      "pnode2",
			SlurmNodeName:    "snode2",
		},
	}
	assert.True(t, s1.Status.SlurmJobRunCurrentStatus.IsEqual(s2.Status.SlurmJobRunCurrentStatus))

	s1.Status.SlurmJobRunCurrentStatus.PhysicalNodeSnapshots["node3"] = &PhysicalNodeSnapshot{
		PhysicalNodeName: "node3",
		K8sNodeName:      "pnode3",
		SlurmNodeName:    "snode3",
	}
	assert.False(t, s1.Status.SlurmJobRunCurrentStatus.IsEqual(s2.Status.SlurmJobRunCurrentStatus))
}
