package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
)

func (r *PhysicalNodeReconciler) SyncSlurmAndK8sNodeSpecAndStatus(
	ctx context.Context,
	slurmNodeMap map[string]*slurm.SlurmNode,
	slurmPodMap map[string]*corev1.Pod,
	k8sNodeMap map[string]*corev1.Node,
	existingPhysicalNodeMap map[string]*slonkv1.PhysicalNode,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started syncing slurm and k8s nodes spect and status")

	// Construct a map of fresh physical node statuses based on slurm and k8s data.
	freshPhysicalNodeStatusMap := map[string]*slonkv1.PhysicalNodeStatus{}
	slurmCount := 0
	for _, slurmNode := range slurmNodeMap {
		// logger.Info("Processing slurm node", "name", slurmNode.Name, "state", slurmNode.State, "comment", slurmNode.Comment, "reason", slurmNode.Reason)

		// Get k8s node, assuming slurm internal node name = pod name.
		pod, ok := slurmPodMap[slurmNode.Name]
		if !ok {
			// Log and continue.
			logger.Info("No pod found for slurm node", "name", slurmNode.Name)
			continue
		}
		if pod.Spec.NodeName == "" {
			// Pending pod, continue.
			continue
		}

		k8sNode, ok := k8sNodeMap[pod.Spec.NodeName]
		if !ok {
			// Log and continue.
			logger.Info("No k8s node found for pod", "name", pod.Name, "nodeName", pod.Spec.NodeName)
		}
		physicalHostName, err := r.getPhysicalNodeName(k8sNode, IDENTIFIER_GPU_UUID_HASH)
		if err != nil || physicalHostName == "" {
			// Log and continue.
			if !strings.Contains(slurmNode.Name, "cpu") {
				logger.Info("No physical host name found for slurm node", "name", slurmNode.Name)
			}
			continue
		}
		slurmCount++
		freshPhysicalNodeStatusMap[physicalHostName] = r.constructPhysicalNodeStatus(physicalHostName, slurmNode, k8sNode)
	}
	runningSlurmPodCount := 0
	unknownSlurmNodeCount := 0
	for _, slurmPod := range slurmPodMap {
		for key, value := range slurmPod.Labels {
			if key == "slurm-compute" && value == "yes" && !strings.Contains(slurmPod.Name, "cpu") {
				if slurmPod.Status.Phase == corev1.PodRunning {
					runningSlurmPodCount++
					if _, ok := slurmNodeMap[slurmPod.Name]; !ok {
						unknownSlurmNodeCount++
						logger.Info("No slurm node found for running pod", "slurmPodName", slurmPod.Name, "slurmNodeName", slurmPod.Spec.NodeName)
					}
				}
			}
		}
	}

	// Also add those k8s node without slurm node running at the moment, but have physical node annotation.
	k8sCount := 0
	for _, k8sNode := range k8sNodeMap {
		physicalNodeName, err := r.getPhysicalNodeName(k8sNode, IDENTIFIER_GPU_UUID_HASH)
		if err != nil || physicalNodeName == "" {
			continue
		}
		if _, ok := freshPhysicalNodeStatusMap[physicalNodeName]; !ok {
			k8sCount++
			freshPhysicalNodeStatusMap[physicalNodeName] = r.constructPhysicalNodeStatus(physicalNodeName, nil, k8sNode)
		}
	}

	logger.Info(
		"Constructed fresh physical node statuses total",
		"totalCount", len(freshPhysicalNodeStatusMap),
		"k8sCount", k8sCount,
		"slurmCount", slurmCount,
		"runningSlurmPodCount", runningSlurmPodCount,
		"unknownSlurmNodeCount", unknownSlurmNodeCount,
	)

	// Compare existing and fresh nodes, update, create or mark as inactive as needed.
	updateCount := 0
	for physicalHostName, freshPhysicalNodeStatus := range freshPhysicalNodeStatusMap {
		if existingPhysicalNode, ok := existingPhysicalNodeMap[physicalHostName]; ok {
			// Update existing node if anything changed.
			updatedNode, err := r.maybeUpdatePhysicalNode(ctx, physicalHostName, existingPhysicalNode, freshPhysicalNodeStatus)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("update physical node: %w", err)
			}
			if updatedNode != nil {
				// logger.Info("Updated physical node", "name", updatedNode.Name, "spec", updatedNode.Spec, "status", updatedNode.Status)
				updateCount++
			}
		} else {
			// Create new node.
			// Note: Creation would ignore and overwrite status field, so we need to update status after creation.
			updatedNode, err := r.maybeUpdatePhysicalNode(ctx, physicalHostName, nil, freshPhysicalNodeStatus)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("create physical node: %w", err)
			}
			if updatedNode != nil {
				logger.Info("Created new physical node", "name", updatedNode.Name, "spec", updatedNode.Spec, "status", updatedNode.Status)
			} else {
				logger.Info("skipped creating new physical node due to incomplate data", "name", physicalHostName)
			}
		}
	}

	for physicalHostName, existingPhysicalNode := range existingPhysicalNodeMap {
		// Check if the physical node is removed from slurm and k8s.
		if _, ok := freshPhysicalNodeStatusMap[physicalHostName]; !ok {
			updatedNode, err := r.maybeUpdatePhysicalNode(ctx, physicalHostName, existingPhysicalNode, nil)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("update physical node: %w", err)
			}
			if updatedNode != nil {
				logger.Info("Marked physical node as removed", "name", updatedNode.Name)
				updateCount++
			}
		}
	}

	logger.Info("Finished syncing slurm and k8s nodes spec and status", "updateCount", updateCount)

	return ctrl.Result{}, nil
}

func (r *PhysicalNodeReconciler) getPhysicalNodeName(
	k8sNode *corev1.Node, identifier string,
) (string, error) {
	physicalNodeName := ""
	if k8sNode == nil {
		return "", fmt.Errorf("empty k8s node")
	}
	if identifier == IDENTIFIER_GPU_UUID_HASH {
		var ok bool
		physicalNodeName, ok = k8sNode.Annotations[GPU_UUID_HASH_ANNOTATION]
		if !ok || physicalNodeName == "" {
			return "", fmt.Errorf("no slurm node and no physical host annotation")
		}
	} else if identifier == IDENTIFIER_PHYSICAL_HOST {
		var ok bool
		physicalNodeName, ok = k8sNode.Annotations[PHYSICAL_HOST_ANNOTATION]
		if !ok || physicalNodeName == "" {
			return "", fmt.Errorf("no slurm node and no physical host annotation")
		}
	} else {
		return "", fmt.Errorf("invalid identifier")
	}
	return physicalNodeName, nil
}

func (r *PhysicalNodeReconciler) constructPhysicalNodeStatus(
	physicalHostName string,
	slurmNode *slurm.SlurmNode,
	k8sNode *corev1.Node,
) *slonkv1.PhysicalNodeStatus {
	var slurmNodeStatus slonkv1.SlurmNodeStatus
	if slurmNode != nil {
		slurmNodeStatus = slonkv1.SlurmNodeStatus{
			Name:     slurmNode.Name,
			State:    slurmNode.State,
			Features: slurmNode.Features,
			Comment:  slurmNode.Comment,
			Reason:   slurmNode.Reason,

			Removed:   false,
			Timestamp: v1.Now(),
		}
	} else {
		// If slurm node is not found, mark slurm node as removed.
		slurmNodeStatus = slonkv1.SlurmNodeStatus{
			Name:      "",
			Removed:   true,
			Timestamp: v1.Now(),
		}
	}

	var k8sNodeStatus slonkv1.K8sNodeStatus
	if k8sNode != nil {
		k8sNodeStatus = slonkv1.K8sNodeStatus{
			Name:          k8sNode.Name,
			Unschedulable: k8sNode.Spec.Unschedulable,
			Taints:        k8sNode.Spec.Taints,

			Removed:   false,
			Timestamp: v1.Now(),
		}
	} else {
		// If k8s node is not found, mark k8s node as removed.
		k8sNodeStatus = slonkv1.K8sNodeStatus{
			Name:      "",
			Removed:   true,
			Timestamp: v1.Now(),
		}
	}

	// Construct fresh physical node.
	freshPhysicalNodeStatus := slonkv1.PhysicalNodeStatus{
		SlurmNodeStatus:        slurmNodeStatus,
		SlurmNodeStatusHistory: []slonkv1.SlurmNodeStatus{},
		K8sNodeStatus:          k8sNodeStatus,
		K8sNodeStatusHistory:   []slonkv1.K8sNodeStatus{},
	}

	return &freshPhysicalNodeStatus
}

func (r *PhysicalNodeReconciler) maybeUpdatePhysicalNode(
	ctx context.Context,
	physicalNodeName string,
	existingPhysicalNode *slonkv1.PhysicalNode,
	freshPhysicalNodeStatus *slonkv1.PhysicalNodeStatus,
) (*slonkv1.PhysicalNode, error) {
	logger := log.FromContext(ctx)

	var updatedPhysicalNode *slonkv1.PhysicalNode
	var existingSpec *slonkv1.PhysicalNodeSpec
	if existingPhysicalNode != nil {
		existingSpec = &existingPhysicalNode.Spec
	}
	var existingStatus *slonkv1.PhysicalNodeStatus
	if existingPhysicalNode != nil {
		existingStatus = &existingPhysicalNode.Status
	}

	updatedSpec := r.maybeUpdatePhysicalNodeSpec(existingSpec, freshPhysicalNodeStatus)
	if updatedSpec != nil {
		if existingPhysicalNode == nil {
			updatedPhysicalNode = &slonkv1.PhysicalNode{
				ObjectMeta: v1.ObjectMeta{
					Name:      physicalNodeName,
					Namespace: SLURM_NAMESPACE,
				},
				Spec: *updatedSpec,
			}
			if err := r.Client.Create(ctx, updatedPhysicalNode); err != nil {
				return nil, fmt.Errorf("create physical node: %w", err)
			}
		} else {
			logger.Info("Updating physical node spec", "name", physicalNodeName, "spec", updatedSpec)
			updatedPhysicalNode = existingPhysicalNode.DeepCopy()
			updatedPhysicalNode.Spec = *updatedSpec
			if err := r.Client.Update(ctx, updatedPhysicalNode); err != nil {
				return nil, fmt.Errorf("update physical node spec: %w", err)
			}
		}
	}

	updatedStatus, removedSlurmNode, removedK8sNode := r.maybeUpdatePhysicalNodeStatus(
		existingStatus, freshPhysicalNodeStatus)
	if updatedStatus != nil {
		if updatedPhysicalNode == nil {
			updatedPhysicalNode = existingPhysicalNode.DeepCopy()
		}

		updatedPhysicalNode.Status = *updatedStatus
		if err := r.Client.Status().Update(ctx, updatedPhysicalNode); err != nil {
			return nil, fmt.Errorf("update physical node slurm node status: %w", err)
		}
		if removedSlurmNode != "" {
			matchingEventRecord := false
			for _, eventRecord := range updatedPhysicalNode.EventRecords {
				if eventRecord.Event.Reason == REASON_SLONKLET_AUTO_SLURM_NODE_DELETION &&
					strings.Contains(eventRecord.Event.Message, removedSlurmNode) &&
					eventRecord.AckTimestamp.Add(1*time.Hour).After(time.Now()) {
					matchingEventRecord = true
				}
			}

			if matchingEventRecord {
				logger.Info(
					"Event record already exists for slurm node removal, skipping event emission",
					"slurm node", removedSlurmNode,
					"physical node", physicalNodeName,
				)
			} else if updatedPhysicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDown {
				// Something removed it before we could, though it should be removed anyway.
				logger.Info("Emitting event for passive slurm node removal", "slurm node", removedSlurmNode, "physical node", physicalNodeName)

				message := fmt.Sprintf("Passive removal of slurm node %s. Physical node: %s.", removedSlurmNode, physicalNodeName)
				if err := r.emitAndRecordEvent(
					updatedPhysicalNode,
					REASON_SLONKLET_AUTO_SLURM_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit slurm node removal event", "error", err)
				}
			} else {
				logger.Info("Emitting event for unexpected slurm node removal", "slurm node", removedSlurmNode, "physical node", physicalNodeName)

				message := fmt.Sprintf("Unexpected removal of slurm node %s. Physical node: %s.", removedSlurmNode, physicalNodeName)
				if err := r.emitAndRecordEvent(
					updatedPhysicalNode,
					REASON_SLONKLET_UNEXPECTED_SLURM_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit slurm node removal event", "error", err)
				}
			}
		}
		if removedK8sNode != "" {
			matchingEventRecord := false
			for _, eventRecord := range updatedPhysicalNode.EventRecords {
				if (eventRecord.Event.Reason == REASON_SLONKLET_AUTO_K8S_NODE_DELETION ||
					eventRecord.Event.Reason == REASON_SLONKLET_AUTO_K8S_NODE_DRAIN) &&
					strings.Contains(eventRecord.Event.Message, removedK8sNode) &&
					eventRecord.AckTimestamp.Add(1*time.Hour).After(time.Now()) {
					matchingEventRecord = true
				}
			}

			if matchingEventRecord {
				logger.Info(
					"Event record already exists for k8s node removal, skipping event emission",
					"k8s node", removedK8sNode,
					"physical node", physicalNodeName,
				)
			} else if updatedPhysicalNode.Spec.K8sNodeSpec.GoalState == GoalStateDown || updatedPhysicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDown {
				// GKE removed it before we could, though it should be removed anyway.
				// TODO (yiran): We shouldn't care about slurm node goal state here, remove that after improving state machine.
				logger.Info("Emitting event for passive k8s node removal", "k8s node", removedK8sNode, "physical node", physicalNodeName)

				message := fmt.Sprintf("Passive removal of K8s node %s. Physical node: %s.", removedK8sNode, physicalNodeName)
				if err := r.emitAndRecordEvent(
					updatedPhysicalNode,
					REASON_SLONKLET_AUTO_K8S_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit k8s node removal event", "error", err)
				}
			} else {
				logger.Info("Emitting event for unexpected k8s node removal", "k8s node", removedK8sNode, "physical node", physicalNodeName)

				message := fmt.Sprintf("Unexpected removal of K8s node %s. Physical node: %s.", removedK8sNode, physicalNodeName)
				if err := r.emitAndRecordEvent(
					updatedPhysicalNode,
					REASON_SLONKLET_UNEXPECTED_K8S_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit k8s node removal event", "error", err)
				}
			}
		}
	}

	if updatedSpec != nil || updatedStatus != nil {
		return updatedPhysicalNode, nil
	}
	return nil, nil
}

func (r *PhysicalNodeReconciler) maybeUpdatePhysicalNodeSpec(
	existingPhysicalNodeSpec *slonkv1.PhysicalNodeSpec,
	freshPhysicalNodeStatus *slonkv1.PhysicalNodeStatus,
) *slonkv1.PhysicalNodeSpec {
	if existingPhysicalNodeSpec != nil && existingPhysicalNodeSpec.Manual {
		// If physical node is manually managed, keep existing goal state.
		return nil
	}

	// Determine slurm goal state based on slurm node internal state.
	// Set slurm goal state if:
	//  - Slurm node is in DRAIN state
	//  - Not caused by manually execute scontrol reboot or prolog/epilog failures
	//  - Not caused by following goal state enforced by slonklet

	// By default, slurm node is not found and we have no info from physical node, set goal state to true.
	slurmNodeSpec := slonkv1.SlurmNodeSpec{
		GoalState: GoalStateUp,
		Timestamp: v1.Now(),
	}

	if existingPhysicalNodeSpec != nil {
		// Backfill in potentially missing goalstate data.
		if existingPhysicalNodeSpec.SlurmNodeSpec.GoalState == "" {
			existingPhysicalNodeSpec.SlurmNodeSpec.GoalState = GoalStateUp
		}

		// Keep existing goal state.
		slurmNodeSpec = slonkv1.SlurmNodeSpec{
			GoalState: existingPhysicalNodeSpec.SlurmNodeSpec.GoalState,
			Reason:    existingPhysicalNodeSpec.SlurmNodeSpec.Reason,
			Timestamp: v1.Now(),
		}
	}

	manualDrain := false
	if freshPhysicalNodeStatus != nil && freshPhysicalNodeStatus.SlurmNodeStatus.Name != "" {
		if freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "reboot" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "reboot ASAP" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "Reboot ASAP" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "reboot requested" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "Not responding" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "Kill task failed" &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "failed_health_check" &&
			!strings.HasPrefix(freshPhysicalNodeStatus.SlurmNodeStatus.Reason, "Init error") &&
			!strings.HasPrefix(freshPhysicalNodeStatus.SlurmNodeStatus.Reason, "Epilog error") &&
			!strings.HasPrefix(freshPhysicalNodeStatus.SlurmNodeStatus.Reason, "Prolog error") &&
			freshPhysicalNodeStatus.SlurmNodeStatus.Reason != "" {
			for _, state := range freshPhysicalNodeStatus.SlurmNodeStatus.State {
				if state == "DRAIN" {
					manualDrain = true
				} else if state == "REBOOT_REQUESTED" || state == "REBOOT_ISSUED" {
					// Manually rebooted, which will also mark node as DRAIN.
					manualDrain = false
					break
				}
			}
		}
		if manualDrain {
			slurmNodeSpec.GoalState = GoalStateDrain
			if slurmNodeSpec.Reason == "" {
				// TODO (yiran): Maybe update the reason, in case user manually updated the reason in slurm.
				slurmNodeSpec.Reason = freshPhysicalNodeStatus.SlurmNodeStatus.Reason
			}
		}
	}

	// For k8s node, we always set goal state to up, for now.
	k8sNodeSpec := slonkv1.K8sNodeSpec{
		GoalState: GoalStateUp,
		Timestamp: v1.Now(),
	}

	if existingPhysicalNodeSpec == nil ||
		!existingPhysicalNodeSpec.SlurmNodeSpec.IsEqual(slurmNodeSpec) ||
		!existingPhysicalNodeSpec.K8sNodeSpec.IsEqual(k8sNodeSpec) {
		return &slonkv1.PhysicalNodeSpec{
			SlurmNodeSpec: slurmNodeSpec,
			K8sNodeSpec:   k8sNodeSpec,
			Manual:        manualDrain,
		}
	}

	return nil
}

func (r *PhysicalNodeReconciler) maybeUpdatePhysicalNodeStatus(
	existingPhysicalNodeStatus *slonkv1.PhysicalNodeStatus,
	freshPhysicalNodeStatus *slonkv1.PhysicalNodeStatus,
) (*slonkv1.PhysicalNodeStatus, string, string) {
	removedSlurmNode := ""
	removedK8sNode := ""

	if existingPhysicalNodeStatus == nil {
		return freshPhysicalNodeStatus, "", ""
	}

	updateStatus := false
	resultPhysicalNodeStatus := existingPhysicalNodeStatus.DeepCopy()

	if freshPhysicalNodeStatus == nil || freshPhysicalNodeStatus.SlurmNodeStatus.Name == "" {
		// Mark slurm node as removed.
		if !resultPhysicalNodeStatus.SlurmNodeStatus.Removed {
			updateStatus = true
			removedSlurmNode = resultPhysicalNodeStatus.SlurmNodeStatus.Name

			resultPhysicalNodeStatus.SlurmNodeStatusHistory = append(
				[]slonkv1.SlurmNodeStatus{resultPhysicalNodeStatus.SlurmNodeStatus},
				resultPhysicalNodeStatus.SlurmNodeStatusHistory...,
			)
			if len(resultPhysicalNodeStatus.SlurmNodeStatusHistory) > NODE_HISTORY_LENGTH {
				resultPhysicalNodeStatus.SlurmNodeStatusHistory = resultPhysicalNodeStatus.SlurmNodeStatusHistory[:NODE_HISTORY_LENGTH]
			}
			resultPhysicalNodeStatus.SlurmNodeStatus = slonkv1.SlurmNodeStatus{
				Removed:   true,
				Timestamp: v1.Now(),
			}
		}
	} else {
		// Update slurm node status if anything changed.
		if !resultPhysicalNodeStatus.SlurmNodeStatus.IsEqual(freshPhysicalNodeStatus.SlurmNodeStatus) {
			updateStatus = true

			resultPhysicalNodeStatus.SlurmNodeStatusHistory = append(
				[]slonkv1.SlurmNodeStatus{resultPhysicalNodeStatus.SlurmNodeStatus},
				resultPhysicalNodeStatus.SlurmNodeStatusHistory...,
			)
			if len(resultPhysicalNodeStatus.SlurmNodeStatusHistory) > NODE_HISTORY_LENGTH {
				resultPhysicalNodeStatus.SlurmNodeStatusHistory = resultPhysicalNodeStatus.SlurmNodeStatusHistory[:NODE_HISTORY_LENGTH]
			}
			resultPhysicalNodeStatus.SlurmNodeStatus = freshPhysicalNodeStatus.SlurmNodeStatus
		}
	}
	if freshPhysicalNodeStatus == nil || freshPhysicalNodeStatus.K8sNodeStatus.Name == "" {
		// Mark k8s node as removed.
		if !resultPhysicalNodeStatus.K8sNodeStatus.Removed {
			updateStatus = true
			removedK8sNode = resultPhysicalNodeStatus.K8sNodeStatus.Name

			resultPhysicalNodeStatus.K8sNodeStatusHistory = append(
				[]slonkv1.K8sNodeStatus{resultPhysicalNodeStatus.K8sNodeStatus},
				resultPhysicalNodeStatus.K8sNodeStatusHistory...,
			)
			if len(resultPhysicalNodeStatus.K8sNodeStatusHistory) > NODE_HISTORY_LENGTH {
				resultPhysicalNodeStatus.K8sNodeStatusHistory = resultPhysicalNodeStatus.K8sNodeStatusHistory[:NODE_HISTORY_LENGTH]
			}
			resultPhysicalNodeStatus.K8sNodeStatus = slonkv1.K8sNodeStatus{
				Removed:   true,
				Timestamp: v1.Now(),
			}
		}
	} else {
		// Update k8s node status if anything changed.
		if !resultPhysicalNodeStatus.K8sNodeStatus.IsEqual(freshPhysicalNodeStatus.K8sNodeStatus) {
			resultPhysicalNodeStatus.K8sNodeStatusHistory = append(
				[]slonkv1.K8sNodeStatus{resultPhysicalNodeStatus.K8sNodeStatus},
				resultPhysicalNodeStatus.K8sNodeStatusHistory...,
			)
			if len(resultPhysicalNodeStatus.K8sNodeStatusHistory) > NODE_HISTORY_LENGTH {
				resultPhysicalNodeStatus.K8sNodeStatusHistory = resultPhysicalNodeStatus.K8sNodeStatusHistory[:NODE_HISTORY_LENGTH]
			}
			resultPhysicalNodeStatus.K8sNodeStatus = freshPhysicalNodeStatus.K8sNodeStatus
			updateStatus = true
		}
	}

	if updateStatus {
		return resultPhysicalNodeStatus, removedSlurmNode, removedK8sNode
	}
	return nil, removedSlurmNode, removedK8sNode
}
