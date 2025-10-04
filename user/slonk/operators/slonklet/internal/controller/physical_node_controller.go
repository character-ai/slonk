/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
)

const (
	NGINX_INGRESS_NAMESPACE  = "ingress-nginx"
	SLURM_NAMESPACE          = "slurm"
	IDENTIFIER_GPU_UUID_HASH = "gpu-uuid-hash"
	IDENTIFIER_PHYSICAL_HOST = "physical-host"
	PHYSICAL_HOST_ANNOTATION = "slonk.your-org.com/physical-host"
	GPU_UUID_HASH_ANNOTATION = "slonk.your-org.com/gpu-uuid-hash"

	K8S_GOAL_STATE_ANNOTATION = "slonk.your-org.com/k8s-goal-state"

	SLURM_GOAL_STATE_ANNOTATION = "slonk.your-org.com/slurm-goal-state"
	SLURM_REASON_ANNOTATION     = "slonk.your-org.com/slurm-reason"

	SLURM_TAINT_PREFIX        = "slonk.your-org.com/"
	SLURM_TAINT_GOAL_STATE    = "slonk.your-org.com/slurm-goal-state"
	SLURM_TAINT_ACTION_QUIT   = "slonk.your-org.com/action-quit"
	SLURM_TAINT_ACTION_REBOOT = "slonk.your-org.com/action-reboot"
	SLURM_TAINT_ACTION_MANUAL = "slonk.your-org.com/action-manual"
	SLURM_TAINT_ACTION_RMA    = "slonk.your-org.com/action-rma"

	NODE_HISTORY_LENGTH       = 50
	TAINT_LIMIT_PER_ITERATION = 100
	TAINT_LIMIT_TOTAL         = 100
)

// PhysicalNodeReconciler reconciles a PhysicalNode object
type PhysicalNodeReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

//+kubebuilder:rbac:groups=slonk.your-org.com,resources=physicalnodes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=slonk.your-org.com,resources=physicalnodes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=slonk.your-org.com,resources=physicalnodes/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",namespace=slurm,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PhysicalNode object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *PhysicalNodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	return ctrl.Result{}, nil
}

func (r *PhysicalNodeReconciler) Sync(
	ctx context.Context,
	socketPath string,
	autoRemediate bool,
) (map[string]*slonkv1.PhysicalNode, error) {
	logger := log.FromContext(ctx)

	logger.Info("----------")
	logger.Info("Started syncing physical nodes")

	slurmNodeList, err := slurm.ListSlurmNodes(socketPath)
	if err != nil {
		return nil, fmt.Errorf("fetch slurm nodes: %w", err)
	}
	slurmNodeMap := map[string]*slurm.SlurmNode{}
	for _, slurmNode := range slurmNodeList {
		slurmNodeCopy := slurmNode // Copy to avoid pointer reuse.
		slurmNodeMap[slurmNode.Name] = &slurmNodeCopy
	}

	slurmPodList := corev1.PodList{}
	if err := r.Client.List(ctx, &slurmPodList, client.InNamespace(SLURM_NAMESPACE)); err != nil {
		return nil, fmt.Errorf("list slurm pods: %w", err)
	}
	slurmPodMap := map[string]*corev1.Pod{}
	for _, slurmPod := range slurmPodList.Items {
		slurmPodCopy := slurmPod // Copy to avoid pointer reuse.
		slurmPodMap[slurmPod.Name] = &slurmPodCopy
	}

	k8sNodeList := corev1.NodeList{}
	if err := r.Client.List(ctx, &k8sNodeList); err != nil {
		return nil, fmt.Errorf("list k8s nodes: %w", err)
	}
	k8sNodeMap := map[string]*corev1.Node{}
	for _, k8sNode := range k8sNodeList.Items {
		k8sNodeCopy := k8sNode // Copy to avoid pointer reuse.
		k8sNodeMap[k8sNode.Name] = &k8sNodeCopy
	}

	existingPhysicalNodeList := slonkv1.PhysicalNodeList{}
	if err := r.Client.List(ctx, &existingPhysicalNodeList); err != nil {
		return nil, fmt.Errorf("list physical nodes: %w", err)
	}
	existingPhysicalNodeMap := map[string]*slonkv1.PhysicalNode{}
	for _, existingPhysicalNode := range existingPhysicalNodeList.Items {
		existingPhysicalNodeCopy := existingPhysicalNode // Copy to avoid pointer reuse.
		existingPhysicalNodeMap[existingPhysicalNode.Name] = &existingPhysicalNodeCopy
	}
	logger.Info("Fetched data",
		"slurm node map", len(slurmNodeMap),
		"slurm pod map", len(slurmPodMap),
		"k8s node map", len(k8sNodeMap),
		"physical node map", len(existingPhysicalNodeMap),
	)

	if _, err := r.SyncSlurmAndK8sNodeSpecAndStatus(ctx, slurmNodeMap, slurmPodMap, k8sNodeMap, existingPhysicalNodeMap); err != nil {
		return nil, fmt.Errorf("sync slurm and k8s node specs and statuses: %w", err)
	}

	// TODO (yiran): Remove this block.
	if err := r.Client.List(ctx, &existingPhysicalNodeList); err != nil {
		return nil, fmt.Errorf("list physical nodes: %w", err)
	}
	existingPhysicalNodeMap = map[string]*slonkv1.PhysicalNode{}
	for _, existingPhysicalNode := range existingPhysicalNodeList.Items {
		existingPhysicalNodeCopy := existingPhysicalNode // Copy to avoid pointer reuse.
		existingPhysicalNodeMap[existingPhysicalNode.Name] = &existingPhysicalNodeCopy
	}

	// if _, err := r.PropogateSlurmReservationToK8sNodeTaints(ctx, k8sNodeMap); err != nil {
	// 	return nil, fmt.Errorf("propogate k8s goal state to slurm node annotations: %w", err)
	// }

	if _, err := r.PropogateSlurmGoalStateToK8sNodeTaints(ctx, slurmNodeMap, k8sNodeMap, existingPhysicalNodeMap); err != nil {
		return nil, fmt.Errorf("propogate slurm goal state to k8s node taints: %w", err)
	}

	if autoRemediate {
		if _, err := r.AutoRemediate(ctx, slurmNodeMap, k8sNodeMap, existingPhysicalNodeMap, false); err != nil {
			return nil, fmt.Errorf("auto-remediate k8s nodes: %w", err)
		}
	}

	logger.Info("Finished syncing physical nodes")

	return existingPhysicalNodeMap, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PhysicalNodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&slonkv1.PhysicalNode{}).
		Complete(r)
}
