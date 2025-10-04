package tools

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AddTaintToNode adds a new taint to a specific node.
func MaybeAddTaintToNode(
	ctx context.Context,
	cli client.Client,
	node *corev1.Node,
	taint corev1.Taint,
) (bool, error) {
	exists := false
	for _, nodeTaint := range node.Spec.Taints {
		if nodeTaint.Key == taint.Key && nodeTaint.Value == taint.Value && nodeTaint.Effect == taint.Effect {
			exists = true
			break
		}
	}

	if !exists {
		node.Spec.Taints = append(node.Spec.Taints, taint)
		if err := cli.Update(ctx, node); err != nil {
			return false, fmt.Errorf("update node taint: %w", err)
		}
	}

	return !exists, nil
}

func MaybeRemoveTaintFromNode(
	ctx context.Context,
	cli client.Client,
	node *corev1.Node,
	taint corev1.Taint,
) (bool, error) {
	exists := false
	for i, nodeTaint := range node.Spec.Taints {
		if nodeTaint.Key == taint.Key && nodeTaint.Value == taint.Value && nodeTaint.Effect == taint.Effect {
			node.Spec.Taints = append(node.Spec.Taints[:i], node.Spec.Taints[i+1:]...)
			exists = true
			break
		}
	}

	if exists {
		if err := cli.Update(ctx, node); err != nil {
			return false, fmt.Errorf("update node taint: %w", err)
		}
	}

	return exists, nil
}

func DeletePodsOnNode(
	ctx context.Context,
	cli client.Client,
	podLists map[string]corev1.PodList,
	k8sNodeName string,
) (bool, error) {
	deleted := false
	// If the k8s node is tainted, evict pods on the node.
	for _, podList := range podLists {
		for _, pod := range podList.Items {
			if pod.Spec.NodeName == k8sNodeName && pod.DeletionTimestamp == nil {
				if err := cli.Delete(ctx, &pod, client.Preconditions{
					UID: &pod.UID,
				}, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
					return false, fmt.Errorf("delete pod: %w", err)
				} else {
					deleted = true
				}
			}
		}
	}

	return deleted, nil
}

func MakeEvent(ref *v1.ObjectReference, annotations map[string]string, eventtype, reason, message string, sourceComponent string) *v1.Event {
	t := metav1.Now()
	namespace := ref.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%v.%x", ref.Name, t.UnixNano()),
			Namespace:   namespace,
			Annotations: annotations,
		},
		InvolvedObject:      *ref,
		Reason:              reason,
		Message:             message,
		FirstTimestamp:      t,
		LastTimestamp:       t,
		Count:               1,
		Type:                eventtype,
		ReportingController: sourceComponent,
		Source: v1.EventSource{
			Component: sourceComponent,
		},
	}
}
