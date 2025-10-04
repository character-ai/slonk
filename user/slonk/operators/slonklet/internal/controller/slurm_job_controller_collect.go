package controller

import (
	"context"
	"fmt"
	"time"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	JOB_HISTORY_LENGTH = 10
)

func (r *SlurmJobReconciler) SyncSlurmJobs(
	ctx context.Context,
	rawSlurmJobMap map[int]*slurm.SlurmJob,
	existingSlurmJobMap map[int]*slonkv1.SlurmJob,
	physicalNodeMap map[string]*slonkv1.PhysicalNode,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started syncing slurm job crds")

	slurmPodToPhysicalNodeMap := map[string]string{}
	for _, physicalNode := range physicalNodeMap {
		if physicalNode.Status.SlurmNodeStatus.Name != "" {
			slurmPodToPhysicalNodeMap[physicalNode.Status.SlurmNodeStatus.Name] = physicalNode.Name
		}
	}

	statusUpdateCount := 0
	freshSlurmJobMap := map[int]*slonkv1.SlurmJob{}
	for rawSlurmJobID, rawSlurmJob := range rawSlurmJobMap {
		physicalNodeSnapshots := map[string]*slonkv1.PhysicalNodeSnapshot{}
		nodeList, err := slurm.ParseJobNodeList(rawSlurmJob.Nodes)
		if err != nil {
			logger.Error(err, "Failed to parse slurm job node list", "job id", rawSlurmJobID)
		}

		submitTime := time.Unix(int64(rawSlurmJob.SubmitTime.Number), 0)
		startTime := time.Unix(int64(rawSlurmJob.StartTime.Number), 0)

		for _, node := range nodeList {
			physicalNodeName, ok := slurmPodToPhysicalNodeMap[node]
			if !ok {
				logger.Info(
					"Physical node not found for slurm job allocated node",
					"job id", rawSlurmJobID,
					"node name", node)
				physicalNodeSnapshots[node] = &slonkv1.PhysicalNodeSnapshot{
					PhysicalNodeName: "UNKNOWN",
					K8sNodeName:      "UNKNOWN",
					SlurmNodeName:    node,
				}
			} else {
				physicalNode := physicalNodeMap[physicalNodeName]
				physicalNodeSnapshots[node] = &slonkv1.PhysicalNodeSnapshot{
					PhysicalNodeName: physicalNodeName,
					K8sNodeName:      physicalNode.Status.K8sNodeStatus.Name,
					SlurmNodeName:    physicalNode.Status.SlurmNodeStatus.Name,
				}
			}
		}

		newSlurmJob := &slonkv1.SlurmJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%d", rawSlurmJobID),
				Namespace: SLURM_NAMESPACE,
			},
			Spec: slonkv1.SlurmJobSpec{
				UserName: rawSlurmJob.UserName,
				Command:  rawSlurmJob.Command,
				Comment:  rawSlurmJob.Comment,
			},
			Status: slonkv1.SlurmJobStatus{
				RestartCount: rawSlurmJob.RestartCount,
				SlurmJobRunCurrentStatus: slonkv1.SlurmJobRunStatus{
					RunID:                 rawSlurmJob.RestartCount,
					Priority:              rawSlurmJob.Priority.Number,
					State:                 rawSlurmJob.JobState,
					SubmitTimestamp:       metav1.NewTime(submitTime),
					StartTimestamp:        metav1.NewTime(startTime),
					LastSyncTimestamp:     metav1.Now(),
					PhysicalNodeSnapshots: physicalNodeSnapshots,
				},
				SlurmJobRunStatusHistory: []slonkv1.SlurmJobRunStatus{},
			},
		}
		freshSlurmJobMap[rawSlurmJobID] = newSlurmJob

		existingSlurmJob, ok := existingSlurmJobMap[rawSlurmJobID]
		if ok {
			if updatedStatus := r.maybeUpdateSlurmJobStatus(&existingSlurmJob.Status, &newSlurmJob.Status); updatedStatus != nil {
				updatedSlurmJob := existingSlurmJob.DeepCopy()
				updatedSlurmJob.Status = *updatedStatus
				if err := r.Client.Status().Update(ctx, updatedSlurmJob); err != nil {
					return ctrl.Result{}, fmt.Errorf("update slurm job: %w", err)
				}
				statusUpdateCount++
			}
		} else {
			if err := r.Client.Create(ctx, newSlurmJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("create slurm job: %w", err)
			}
			if err := r.Client.Status().Update(ctx, newSlurmJob); err != nil {
				return ctrl.Result{}, fmt.Errorf("update slurm job: %w", err)
			}
			logger.Info("Created new slurm job", "job id", rawSlurmJobID)
		}

	}

	for id, existingSlurmJob := range existingSlurmJobMap {
		if _, ok := freshSlurmJobMap[id]; !ok {
			if updatedStatus := r.maybeUpdateSlurmJobStatus(&existingSlurmJob.Status, nil); updatedStatus != nil {
				updatedSlurmJob := existingSlurmJob.DeepCopy()
				updatedSlurmJob.Status = *updatedStatus
				if err := r.Client.Status().Update(ctx, updatedSlurmJob); err != nil {
					return ctrl.Result{}, fmt.Errorf("mark slurm job as removed: %w", err)
				}
				logger.Info("Mark slurm job as removed", "job id", id)
				statusUpdateCount++
			}
		}
	}

	logger.Info("Finished syncing slurm job crds", "status update count", statusUpdateCount)

	return ctrl.Result{}, nil
}

func (r *SlurmJobReconciler) maybeUpdateSlurmJobStatus(
	existingSlurmJobStatus *slonkv1.SlurmJobStatus,
	freshSlurmJobStatus *slonkv1.SlurmJobStatus,
) *slonkv1.SlurmJobStatus {
	if existingSlurmJobStatus == nil {
		return freshSlurmJobStatus
	}

	updateStatus := false
	resultSlurmJobStatus := existingSlurmJobStatus.DeepCopy()
	if freshSlurmJobStatus != nil {
		if !resultSlurmJobStatus.SlurmJobRunCurrentStatus.IsEqual(freshSlurmJobStatus.SlurmJobRunCurrentStatus) {
			resultSlurmJobStatus.SlurmJobRunStatusHistory = append(
				[]slonkv1.SlurmJobRunStatus{resultSlurmJobStatus.SlurmJobRunCurrentStatus},
				resultSlurmJobStatus.SlurmJobRunStatusHistory...,
			)
			if len(resultSlurmJobStatus.SlurmJobRunStatusHistory) > JOB_HISTORY_LENGTH {
				resultSlurmJobStatus.SlurmJobRunStatusHistory = resultSlurmJobStatus.SlurmJobRunStatusHistory[:JOB_HISTORY_LENGTH]
			}
			resultSlurmJobStatus.SlurmJobRunCurrentStatus = freshSlurmJobStatus.SlurmJobRunCurrentStatus
			updateStatus = true
		}

		// Sum up the accumulated runtime.
		// physicalNodeAccumulatedRuntime := map[string]time.Duration{}
		// for _, snapshot := range resultSlurmJobStatus.SlurmJobRunCurrentStatus.PhysicalNodeSnapshots {
		// 	physicalNodeAccumulatedRuntime[snapshot.PhysicalNodeName] += time.Since(resultSlurmJobStatus.SlurmJobRunCurrentStatus.StartTimestamp.Time)
		// }
		// for _, status := range resultSlurmJobStatus.SlurmJobRunStatusHistory {
		// 	for _, snapshot := range status.PhysicalNodeSnapshots {
		// 		physicalNodeAccumulatedRuntime[snapshot.PhysicalNodeName] += status.LastSyncTimestamp.Sub(status.StartTimestamp.Time)
		// 	}
		// }
		// for _, node := range resultSlurmJobStatus.SlurmJobRunCurrentStatus.PhysicalNodeSnapshots {
		// 	if _, ok := physicalNodeAccumulatedRuntime[node.PhysicalNodeName]; !ok {
		// 		node.AccumulatedRuntime = metav1.Duration{Duration: physicalNodeAccumulatedRuntime[node.PhysicalNodeName]}
		// 	}
		// }
	} else {
		// If the fresh status is nil, it means the job is not found in the slurm cluster, probably completed.
		if !resultSlurmJobStatus.SlurmJobRunCurrentStatus.Removed {
			resultSlurmJobStatus.SlurmJobRunStatusHistory = append(
				resultSlurmJobStatus.SlurmJobRunStatusHistory,
				resultSlurmJobStatus.SlurmJobRunCurrentStatus,
			)
			if len(resultSlurmJobStatus.SlurmJobRunStatusHistory) > JOB_HISTORY_LENGTH {
				resultSlurmJobStatus.SlurmJobRunStatusHistory = resultSlurmJobStatus.SlurmJobRunStatusHistory[:JOB_HISTORY_LENGTH]
			}
			resultSlurmJobStatus.SlurmJobRunCurrentStatus = slonkv1.SlurmJobRunStatus{
				Removed:           true,
				LastSyncTimestamp: v1.Now(),
			}
			updateStatus = true
		}
	}

	if updateStatus {
		return resultSlurmJobStatus
	}
	return nil
}
