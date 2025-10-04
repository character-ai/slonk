package controller

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
	"your-org.com/slonklet/internal/tools"
)

const (
	EVICTION_EVENT_REASON = "SlonkletSlurmPodEviction"

	GCP_MAINTENANCE_STARTED               = "cloud.google.com/maintenance-window-started"
	GCP_MAINTENANCE_IMPENDING_TERMINATION = "cloud.google.com/impending-node-termination"

	ACTION_SLURM_POD_KEEP    = "SlurmPodKeep"
	ACTION_SLURM_POD_DELETE  = "SlurmPodDelete"
	ACTION_SLURM_POD_RESTART = "SlurmPodRestart"

	ACTION_K8S_NODE_KEEP   = "K8sNodeKeep"
	ACTION_K8S_NODE_DRAIN  = "K8sNodeDrain"
	ACTION_K8S_NODE_DELETE = "K8sNodeDelete"
)

func (r *PhysicalNodeReconciler) PropogateSlurmReservationToK8sNodeTaints(
	ctx context.Context,
	k8sNodeMap map[string]*corev1.Node,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	SLURM_RESERVATION_TAINT_KEY := "reservation"
	SLURM_RESERVATION_TAINT_VALUE := "hero"
	K8S_NODE_REGEX := ".*-cluster-h100-(([1-6]|1[2-3]))-.*" // This is an example regex for matching specific node patterns.
	re, err := regexp.Compile(K8S_NODE_REGEX)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("compile regex: %w", err)
	}

	logger.Info("Started tainting k8s nodes for reservations")

	for _, k8sNode := range k8sNodeMap {
		if !re.MatchString(k8sNode.Name) {
			continue
		}

		toApply := false
		if k8sNode.Spec.Taints == nil {
			toApply = true
		} else {
			toApply = true
			for _, taint := range k8sNode.Spec.Taints {
				if taint.Key == SLURM_RESERVATION_TAINT_KEY && taint.Value == SLURM_RESERVATION_TAINT_VALUE && taint.Effect == corev1.TaintEffectNoSchedule {
					toApply = false
					break
				}
			}
		}
		if !toApply {
			continue
		}

		currentK8sNode := k8sNode.DeepCopy()
		newTaint := corev1.Taint{
			Key:    SLURM_RESERVATION_TAINT_KEY,
			Value:  SLURM_RESERVATION_TAINT_VALUE,
			Effect: corev1.TaintEffectNoSchedule,
		}
		newTaints := []corev1.Taint{}
		for _, taint := range currentK8sNode.Spec.Taints {
			if taint.Key == SLURM_RESERVATION_TAINT_KEY && taint.Value == SLURM_RESERVATION_TAINT_VALUE {
				// Ignore existing taint with same key and value but different effect.
				continue
			}
			newTaints = append(newTaints, taint)
		}
		newTaints = append(newTaints, newTaint)

		currentK8sNode.Spec.Taints = newTaints
		if err := r.Client.Update(ctx, currentK8sNode); err != nil {
			// Log and continue.
			logger.Info(
				"Failed to add taint to k8s node",
				"name", currentK8sNode.Name,
				"error", err,
			)
		} else {
			logger.Info(
				"Added taint to k8s node",
				"name", currentK8sNode.Name,
				"taint", newTaint,
			)
		}
	}

	logger.Info("Finished tainting k8s nodes for reservations")

	return ctrl.Result{}, nil
}

func (r *PhysicalNodeReconciler) PropogateSlurmGoalStateToK8sNodeTaints(
	ctx context.Context,
	slurmNodeMap map[string]*slurm.SlurmNode,
	k8sNodeMap map[string]*corev1.Node,
	existingPhysicalNodeMap map[string]*slonkv1.PhysicalNode,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started tainting k8s nodes for goal states")

	taintCountTotal := 0
	taintCountInIteration := 0

	taintedK8sNodeNameList := []string{}
	for _, k8sNode := range k8sNodeMap {
		if k8sNode.Spec.Taints != nil {
			for _, taint := range k8sNode.Spec.Taints {
				if strings.HasPrefix(taint.Key, SLURM_TAINT_PREFIX) {
					taintCountTotal++
					taintedK8sNodeNameList = append(taintedK8sNodeNameList, k8sNode.Name)

					slonkAnnoations := map[string]string{}
					for k, v := range k8sNode.Annotations {
						if k == SLURM_GOAL_STATE_ANNOTATION || k == SLURM_REASON_ANNOTATION || k == GPU_UUID_HASH_ANNOTATION {
							slonkAnnoations[k] = v
						}
					}
					logger.Info("Fetched tainted k8s node", "name", k8sNode.Name, "taint", taint, "annotations", slonkAnnoations)
					break
				}
			}
		}
	}
	logger.Info("Fetched all tainted k8s nodes", "count", taintCountTotal, "names", taintedK8sNodeNameList)

	for _, existingPhysicalNode := range existingPhysicalNodeMap {
		if existingPhysicalNode.Status.K8sNodeStatus.Name != "" {
			currentK8sNode, ok := k8sNodeMap[existingPhysicalNode.Status.K8sNodeStatus.Name]
			if ok {
				updateAnnotations := false
				updateTaints := false
				var newAnnotation string
				var newTaint corev1.Taint

				if val, ok := currentK8sNode.Annotations[SLURM_GOAL_STATE_ANNOTATION]; !ok || val != existingPhysicalNode.Spec.SlurmNodeSpec.GoalState {
					currentK8sNode.Annotations[SLURM_GOAL_STATE_ANNOTATION] = existingPhysicalNode.Spec.SlurmNodeSpec.GoalState
					updateAnnotations = true
					newAnnotation = fmt.Sprintf("%s:%s", SLURM_GOAL_STATE_ANNOTATION, existingPhysicalNode.Spec.SlurmNodeSpec.GoalState)
					// Also include reason, or clear previous reason.
					if existingPhysicalNode.Spec.SlurmNodeSpec.Reason != "" {
						currentK8sNode.Annotations[SLURM_REASON_ANNOTATION] = existingPhysicalNode.Spec.SlurmNodeSpec.Reason
					} else {
						delete(currentK8sNode.Annotations, SLURM_REASON_ANNOTATION)
					}
				}

				if existingPhysicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDown {
					if taintCountInIteration < TAINT_LIMIT_PER_ITERATION && taintCountTotal < TAINT_LIMIT_TOTAL {
						// If the slurm node goal state is down, taint the k8s node.
						// TODO (yiran): add taints for different goal states.
						taint := corev1.Taint{
							Key:    SLURM_TAINT_GOAL_STATE,
							Value:  GoalStateDown,
							Effect: corev1.TaintEffectNoSchedule,
						}
						exists := false
						for _, nodeTaint := range currentK8sNode.Spec.Taints {
							if nodeTaint.Key == taint.Key && nodeTaint.Value == taint.Value && nodeTaint.Effect == taint.Effect {
								exists = true
								break
							}
						}
						if !exists {
							currentK8sNode.Spec.Taints = append(currentK8sNode.Spec.Taints, taint)
							newTaint = taint
							updateTaints = true
							taintCountInIteration++
							taintCountTotal++
						}
					} else {
						// If we have already tainted enough nodes, stop tainting more nodes in this iteration.
						logger.Info("Reached taint limit", "count in iteration", taintCountInIteration, "count total", taintCountTotal)
					}
				} else if existingPhysicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDrain {
					// TODO (yiran): use scontrol to ensure it's drained.
					slurmNode, ok := slurmNodeMap[existingPhysicalNode.Status.SlurmNodeStatus.Name]
					if !ok {
						logger.Info("Drained slurm node not found for physical node", "name", existingPhysicalNode.Name, "slurm node", existingPhysicalNode.Status.SlurmNodeStatus.Name)
					} else {
						drained := false
						if slurmNode.State != nil {
							for _, state := range slurmNode.State {
								if strings.EqualFold(state, "DRAIN") {
									drained = true
									break
								} else if strings.EqualFold(state, "DOWN") || strings.EqualFold(state, "FUTURE") {
									// This happens during node startups, it's okay.
									logger.Info("Slurm node is supposed to be drained, currently in DOWN or FUTURE", "name", existingPhysicalNode.Name, "slurm node", existingPhysicalNode.Status.SlurmNodeStatus.Name, "state", slurmNode.State)
									drained = true
									break
								}
							}
						}
						if !drained {
							logger.Info("Slurm node is supposed to be in drained state, but isn't", "name", existingPhysicalNode.Name, "slurm node", existingPhysicalNode.Status.SlurmNodeStatus.Name, "state", slurmNode.State)
						}
					}
				}

				// Commit annotations and taints to k8s node.
				if updateAnnotations || updateTaints {
					if err := r.Client.Update(ctx, currentK8sNode); err != nil {
						// Log and continue.
						logger.Info(
							"Failed to add annotations and taints to k8s node",
							"name", currentK8sNode.Name,
							"physical node", existingPhysicalNode.Name,
							"error", err,
						)
					} else {
						if updateTaints {
							logger.Info(
								"Added annotations and taints to k8s node",
								"name", currentK8sNode.Name,
								"physical node", existingPhysicalNode.Name,
								"taint", newTaint,
							)
						} else {
							logger.Info(
								"Added annotations to k8s node",
								"name", currentK8sNode.Name,
								"physical node", existingPhysicalNode.Name,
								"annotation", newAnnotation,
							)
						}
					}
				}
			}
		}
	}

	logger.Info("Finished tainting k8s nodes for goal states")

	return ctrl.Result{}, nil
}

// AutoRemediate deletes one node with known lifecycle taint.
func (r *PhysicalNodeReconciler) AutoRemediate(
	ctx context.Context,
	slurmNodeMap map[string]*slurm.SlurmNode,
	k8sNodeMap map[string]*corev1.Node,
	existingPhysicalNodeMap map[string]*slonkv1.PhysicalNode,
	dryrun bool,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Started auto-remediation for k8s nodes with lifecycle taints")

	slurmPodList := corev1.PodList{}
	if err := r.Client.List(ctx, &slurmPodList, client.InNamespace(SLURM_NAMESPACE)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list slurm pods: %w", err)
	}
	systemPodListUnfiltered := corev1.PodList{}
	if err := r.Client.List(ctx, &systemPodListUnfiltered, client.InNamespace(SYSTEM_NAMESPACE)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list system pods: %w", err)
	}
	systemPodList := corev1.PodList{}
	for _, pod := range systemPodListUnfiltered.Items {
		if len(pod.ObjectMeta.OwnerReferences) > 0 && pod.ObjectMeta.OwnerReferences[0].Kind == "ReplicaSet" {
			systemPodList.Items = append(systemPodList.Items, pod)
		}
	}
	ingressPodList := corev1.PodList{}
	if err := r.Client.List(ctx, &ingressPodList, client.InNamespace(NGINX_INGRESS_NAMESPACE)); err != nil {
		return ctrl.Result{}, fmt.Errorf("list ingress pods: %w", err)
	}
	podLists := map[string]corev1.PodList{
		SLURM_NAMESPACE:         slurmPodList,
		SYSTEM_NAMESPACE:        systemPodList,
		NGINX_INGRESS_NAMESPACE: ingressPodList,
	}

	actionLimit := 30
	actionCount := 0
	for _, k8sNode := range k8sNodeMap {
		if k8sNode.Spec.Taints != nil {
			// Only proceeed if the k8s node has a lifecycle taint.
			var lifecycleTaint *corev1.Taint
			for _, taint := range k8sNode.Spec.Taints {
				if strings.HasPrefix(taint.Key, SLURM_TAINT_PREFIX) ||
					taint.Key == GCP_MAINTENANCE_STARTED ||
					taint.Key == GCP_MAINTENANCE_IMPENDING_TERMINATION {
					lifecycleTaint = taint.DeepCopy()
				}
			}
			if lifecycleTaint == nil {
				continue
			}

			// Find the physical node corresponding to the k8s node.
			physicalNodeName := ""
			var physicalNode *slonkv1.PhysicalNode
			var err error
			var ok bool
			physicalNodeName, err = r.getPhysicalNodeName(k8sNode, IDENTIFIER_GPU_UUID_HASH)
			if err != nil || physicalNodeName == "" {
				if !strings.Contains(k8sNode.Name, "cpu") {
					logger.Info("No physical host name found for k8s node", "name", k8sNode.Name)
				}
				continue
			}
			physicalNode, ok = existingPhysicalNodeMap[physicalNodeName]
			if !ok {
				logger.Info("Physical node not found for k8s node", "name", k8sNode.Name, "physical node", physicalNodeName)
				continue
			}

			// Check if there are running pods. Don't delete the node if there are still running pods on it.
			// Also wait for ~1 hour before deleting the pods if they are already in drain/down state in slurm as intended.
			hasPods := false
			action := ""
			var slurmPod *corev1.Pod
			slurmPodName := ""
			for _, podList := range podLists {
				for _, pod := range podList.Items {
					if pod.Spec.NodeName == k8sNode.Name {
						hasPods = true

						if _, ok := slurmNodeMap[pod.Name]; ok {
							slurmPod = &pod
							slurmPodName = pod.Name

							if lifecycleTaint.Key == SLURM_TAINT_GOAL_STATE {
								for _, state := range slurmNodeMap[pod.Name].State {
									if ((strings.EqualFold(state, "DOWN") && physicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDown) ||
										(strings.EqualFold(state, "DRAIN") && physicalNode.Spec.SlurmNodeSpec.GoalState == GoalStateDrain)) &&
										(pod.Status.StartTime != nil && time.Since(pod.Status.StartTime.Time) < time.Minute*5) {
										logger.Info("Action: keeping slurm pod on tainted k8s node for a while",
											"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
											"pod", pod.Name,
											"k8s node", k8sNode.Name,
											"physical node", physicalNodeName,
										)
										action = ACTION_SLURM_POD_KEEP
										break
									}
								}
								if action == "" {
									logger.Info("Action: delete slurm pod",
										"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
										"pod", pod.Name,
										"k8s node", k8sNode.Name,
										"physical node", physicalNodeName,
									)
									action = ACTION_SLURM_POD_DELETE
								}
							} else if lifecycleTaint.Key == SLURM_TAINT_ACTION_QUIT {
								logger.Info("Action: restart slurm pod",
									"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
									"pod", pod.Name,
									"k8s node", k8sNode.Name,
									"physical node", physicalNodeName,
								)
								action = ACTION_SLURM_POD_RESTART
							} else if lifecycleTaint.Key == SLURM_TAINT_ACTION_REBOOT ||
								lifecycleTaint.Key == SLURM_TAINT_ACTION_MANUAL ||
								lifecycleTaint.Key == SLURM_TAINT_ACTION_RMA {
								logger.Info("Action: delete slurm pod",
									"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
									"pod", pod.Name,
									"k8s node", k8sNode.Name,
									"physical node", physicalNodeName,
								)
								action = ACTION_SLURM_POD_DELETE
							} else if lifecycleTaint.Key == GCP_MAINTENANCE_STARTED ||
								lifecycleTaint.Key == GCP_MAINTENANCE_IMPENDING_TERMINATION {
								logger.Info("Action: keeping slurm pod on tainted k8s node for now after getting maintenance taint",
									"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
									"pod", pod.Name,
									"k8s node", k8sNode.Name,
									"physical node", physicalNodeName,
								)
								action = ACTION_K8S_NODE_KEEP
							} else {
								logger.Info("action: delete slurm pod with unknown taint",
									"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
									"pod", pod.Name,
									"k8s node", k8sNode.Name,
									"physical node", physicalNodeName,
								)
								action = ACTION_SLURM_POD_DELETE
							}

							break
						}
					}
				}
			}

			if action == "" {
				if lifecycleTaint.Key == SLURM_TAINT_ACTION_QUIT {
					// TODO (yiran): this is special case for a corner case.
					logger.Info("Action: untaint k8s node since slurm pod was already deleted",
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
					action = ACTION_SLURM_POD_RESTART
				} else if hasPods || !k8sNode.Spec.Unschedulable {
					logger.Info("Action: drain k8s node",
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
					action = ACTION_K8S_NODE_DRAIN
				} else if time.Since(k8sNode.CreationTimestamp.Time) < time.Minute*15 {
					logger.Info("Action: keeping k8s node with lifecycle taint for a while",
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
					action = ACTION_K8S_NODE_KEEP
				} else {
					logger.Info("Action: delete k8s node",
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
					action = ACTION_K8S_NODE_DELETE
				}
			}

			if dryrun {
				logger.Info("Dryrun, abort action.")
				continue
			}

			if actionCount >= actionLimit {
				logger.Info("Reached action limit", "limit", actionLimit, "count", actionCount)
				break
			}

			// Perform the action.
			acted := false
			if action == ACTION_SLURM_POD_KEEP {
				continue
			} else if action == ACTION_SLURM_POD_RESTART {
				if slurmPod != nil {
					podLists := map[string]corev1.PodList{
						SLURM_NAMESPACE: {Items: []corev1.Pod{*slurmPod}},
					}
					if _, err := tools.DeletePodsOnNode(ctx, r.Client, podLists, k8sNode.Name); err != nil {
						logger.Info("Failed to delete slurm pod",
							"error", err,
							"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
							"pod", slurmPodName,
							"k8s node", k8sNode.Name,
							"physical node", physicalNodeName,
						)
						continue
					}
				}
				k8sNodeCopy := k8sNode.DeepCopy()
				if _, err := tools.MaybeRemoveTaintFromNode(ctx, r.Client, k8sNodeCopy, *lifecycleTaint); err != nil {
					logger.Info("Failed to remove taint from k8s node",
						"error", err,
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"pod", slurmPodName,
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
				}

				acted = true

				message := fmt.Sprintf(
					"Auto untainted k8s node. Lifecycle taint %s. K8s node: %s. Physical node: %s.",
					fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
					k8sNode.Name,
					physicalNodeName)
				if slurmPod != nil {
					message = fmt.Sprintf(
						"Auto removed pod %s for lifecycle taint %s, and untainted k8s node %s. Physical node: %s.",
						slurmPodName,
						fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						k8sNode.Name,
						physicalNodeName)
				}
				if err := r.emitAndRecordEvent(
					physicalNode,
					REASON_SLONKLET_AUTO_SLURM_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit and record eviction event", "error", err)
					continue
				}
			} else if action == ACTION_SLURM_POD_DELETE {
				podLists := map[string]corev1.PodList{
					SLURM_NAMESPACE: {Items: []corev1.Pod{*slurmPod}},
				}
				if _, err := tools.DeletePodsOnNode(ctx, r.Client, podLists, k8sNode.Name); err != nil {
					logger.Info("Failed to delete slurm pod",
						"error", err,
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"pod", slurmPodName,
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
				}

				acted = true

				message := fmt.Sprintf(
					"Auto removed Pod %s for lifecycle taint %s. K8s node: %s. Physical node: %s.",
					slurmPodName,
					fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
					k8sNode.Name,
					physicalNodeName)
				if err := r.emitAndRecordEvent(
					physicalNode,
					REASON_SLONKLET_AUTO_SLURM_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit and record eviction event", "error", err)
					continue
				}
			} else if action == ACTION_K8S_NODE_KEEP {
				continue
			} else if action == ACTION_K8S_NODE_DRAIN {
				k8sNodeCopy := k8sNode.DeepCopy()
				k8sNodeCopy.Spec.Unschedulable = true
				if err := r.Client.Update(ctx, k8sNodeCopy); err != nil {
					logger.Info("Failed to mark k8s node as unschedulable",
						"error", err,
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"pod", slurmPodName,
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
				}

				if _, err := tools.DeletePodsOnNode(ctx, r.Client, podLists, k8sNode.Name); err != nil {
					logger.Info("Failed to evict pods on tainted k8s node",
						"error", err,
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"pod", slurmPodName,
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
				}

				acted = true

				message := fmt.Sprintf(
					"Auto drained K8s node %s for lifecycle taint %s. Physical node: %s.",
					k8sNode.Name,
					fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
					physicalNodeName)
				if err := r.emitAndRecordEvent(
					physicalNode,
					REASON_SLONKLET_AUTO_K8S_NODE_DRAIN,
					message,
				); err != nil {
					logger.Info("Failed to emit and record eviction event", "error", err)
					continue
				}
			} else if action == ACTION_K8S_NODE_DELETE {
				k8sNodeCopy := k8sNode.DeepCopy()
				if err := r.Client.Delete(ctx, k8sNodeCopy); err != nil {
					logger.Info("Failed to delete k8s node with lifecycle taint",
						"error", err,
						"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
						"pod", slurmPodName,
						"k8s node", k8sNode.Name,
						"physical node", physicalNodeName,
					)
				}

				acted = true

				message := fmt.Sprintf(
					"Auto deleted K8s node %s for lifecycle taint %s. Physical node: %s.",
					k8sNode.Name,
					fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
					physicalNodeName)
				if err := r.emitAndRecordEvent(
					physicalNode,
					REASON_SLONKLET_AUTO_K8S_NODE_DELETION,
					message,
				); err != nil {
					logger.Info("Failed to emit and record eviction event", "error", err)
					continue
				}
			} else {
				logger.Info("Unknown action",
					"action", action,
					"taint", fmt.Sprintf("%s:%s", lifecycleTaint.Key, lifecycleTaint.Value),
					"pod", slurmPodName,
					"k8s node", k8sNode.Name,
					"physical node", physicalNodeName)
			}

			// Emit event accordingly.
			if acted {
				actionCount++
			}

		}
	}
	logger.Info("Finished auto-remediation for k8s nodes with lifecycle taints", "actionCount", actionCount)

	return ctrl.Result{}, nil
}
