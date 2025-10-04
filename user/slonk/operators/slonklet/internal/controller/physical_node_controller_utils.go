package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ref "k8s.io/client-go/tools/reference"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/tools"
)

const (
	REASON_SLONKLET_AUTO_SLURM_NODE_DELETION       = "SlonkletAutoSlurmNodeDeletion"
	REASON_SLONKLET_AUTO_K8S_NODE_DRAIN            = "SlonkletAutoK8sNodeDrain"
	REASON_SLONKLET_AUTO_K8S_NODE_DELETION         = "SlonkletAutoK8sNodeDeletion"
	REASON_SLONKLET_UNEXPECTED_SLURM_NODE_DELETION = "SlonkletUnexpectedSlurmNodeDeletion"
	REASON_SLONKLET_UNEXPECTED_K8S_NODE_DELETION   = "SlonkletUnexpectedK8sNodeDeletion"
)

func (r *PhysicalNodeReconciler) emitAndRecordEvent(
	physicalNode *slonkv1.PhysicalNode,
	reason string,
	message string,
) error {
	reference, err := ref.GetReference(r.Scheme, physicalNode)
	if err != nil {
		return fmt.Errorf("get physical node reference: %w", err)
	}
	event := tools.MakeEvent(
		reference,
		map[string]string{},
		corev1.EventTypeNormal,
		reason,
		message,
		"slonklet-controller",
	)
	if err := r.Client.Create(context.Background(), event); err != nil {
		return fmt.Errorf("create event: %w", err)
	}
	physicalNode.EventRecords = append(
		[]slonkv1.EventRecord{
			{
				Event:        *event,
				AckTimestamp: metav1.Now(),
			},
		},
		physicalNode.EventRecords...,
	)
	if len(physicalNode.EventRecords) > 5 {
		physicalNode.EventRecords = physicalNode.EventRecords[:5]
	}
	if err := r.Client.Update(context.Background(), physicalNode); err != nil {
		return fmt.Errorf("update physical node for event records: %w", err)
	}

	return nil
}
