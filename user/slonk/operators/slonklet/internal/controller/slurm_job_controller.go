package controller

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
)

const (
	JOB_TOTAL_LIMIT = 1000
)

// SlurmJobReconciler reconciles a SlurmJob object
type SlurmJobReconciler struct {
	sync.RWMutex

	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=slonk.your-org.com,resources=slurmjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=slonk.your-org.com,resources=slurmjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=slonk.your-org.com,resources=slurmjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=slurm,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SlurmJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *SlurmJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

func (r *SlurmJobReconciler) Sync(
	ctx context.Context,
	socketPath string,
	physicalNodeMap map[string]*slonkv1.PhysicalNode,
) (map[int]*slonkv1.SlurmJob, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started syncing slurm jobs")

	rawSlurmJobList, err := slurm.SyncSlurmJobs(socketPath)
	if err != nil {
		return nil, fmt.Errorf("list slurm jobs: %w", err)
	}
	rawSlurmJobMap := map[int]*slurm.SlurmJob{}
	for _, rawSlurmJob := range rawSlurmJobList {
		rawSlurmJobCopy := rawSlurmJob // Copy to avoid pointer reuse.
		rawSlurmJobMap[rawSlurmJob.JobID] = &rawSlurmJobCopy
	}
	logger.Info("Fetched raw slurm jobs", "list count", len(rawSlurmJobList), "map count", len(rawSlurmJobMap))

	existingSlurmJobList := &slonkv1.SlurmJobList{}
	if err := r.Client.List(ctx, existingSlurmJobList); err != nil {
		return nil, fmt.Errorf("list slurm jobs: %w", err)
	}
	existingSlurmJobMap := map[int]*slonkv1.SlurmJob{}
	for _, existingSlurmJob := range existingSlurmJobList.Items {
		existingSlurmJobCopy := existingSlurmJob // Copy to avoid pointer reuse.
		id, err := strconv.Atoi(existingSlurmJob.ObjectMeta.Name)
		if err != nil {
			logger.Error(err, "Failed to convert slurm job id to int", "job name", existingSlurmJob.ObjectMeta.Name)
			continue
		}
		existingSlurmJobMap[id] = &existingSlurmJobCopy
	}
	logger.Info("Fetched existing slurm jobs", "list count", len(existingSlurmJobList.Items), "map count", len(existingSlurmJobMap))

	if _, err := r.SyncSlurmJobs(ctx, rawSlurmJobMap, existingSlurmJobMap, physicalNodeMap); err != nil {
		return nil, fmt.Errorf("sync slurm jobs: %w", err)
	}

	existingSlurmJobList = &slonkv1.SlurmJobList{}
	if err := r.Client.List(ctx, existingSlurmJobList); err != nil {
		return nil, fmt.Errorf("list slurm jobs: %w", err)
	}
	existingSlurmJobMap = map[int]*slonkv1.SlurmJob{}
	for _, existingSlurmJob := range existingSlurmJobList.Items {
		existingSlurmJobCopy := existingSlurmJob // Copy to avoid pointer reuse.
		id, err := strconv.Atoi(existingSlurmJob.ObjectMeta.Name)
		if err != nil {
			logger.Error(err, "Failed to convert slurm job id to int", "job name", existingSlurmJob.ObjectMeta.Name)
			continue
		}
		existingSlurmJobMap[id] = &existingSlurmJobCopy
	}

	logger.Info("Cleaning up old slurm jobs")
	if len(existingSlurmJobMap) > JOB_TOTAL_LIMIT {
		for len(existingSlurmJobMap) > JOB_TOTAL_LIMIT {
			var oldestID int
			oldestTime := time.Now()
			firstEntry := true
			for id, job := range existingSlurmJobMap {
				if job.Status.SlurmJobRunCurrentStatus.Removed {
					if firstEntry || job.Status.SlurmJobRunCurrentStatus.LastSyncTimestamp.Time.Before(oldestTime) {
						oldestTime = job.Status.SlurmJobRunCurrentStatus.LastSyncTimestamp.Time
						oldestID = id
						firstEntry = false
					}
				}
			}

			if oldestID == 0 {
				break
			}
			// Delete the oldest entry.
			if err := r.Client.Delete(ctx, existingSlurmJobMap[oldestID]); err != nil {
				logger.Error(err, "Failed to delete old slurm job", "job id", oldestID)
				break
			}
			logger.Info("Deleted old slurm job", "job id", oldestID)
			delete(existingSlurmJobMap, oldestID)
		}
	}

	logger.Info("Finished syncing slurm jobs")

	return existingSlurmJobMap, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SlurmJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&slonkv1.SlurmJob{}).
		Complete(r)
}
