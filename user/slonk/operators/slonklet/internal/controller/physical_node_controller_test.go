package controller

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	slonkv1 "your-org.com/slonklet/api/v1"
	"your-org.com/slonklet/internal/slurm"
	// +kubebuilder:scaffold:imports
)

var (
	testPods  []runtime.Object
	testNodes []runtime.Object
)

func setupTest(t *testing.T) {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	testPods = []runtime.Object{
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "slurm-node-1",
				Namespace: SLURM_NAMESPACE,
				Labels:    map[string]string{},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "slurm-node-1",
						Image: "slonk:latest",
					},
				},
				NodeName: "k8s-node-1",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "127.0.0.1",
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:  "slurm-node-1",
						State: corev1.ContainerState{},
					},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "slurm-node-2",
				Namespace: SLURM_NAMESPACE,
				Labels:    map[string]string{},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "slurm-node-2",
						Image: "slonk:latest",
					},
				},
				NodeName: "k8s-node-2",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: "127.0.0.2",
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name:  "slurm-node-2",
						State: corev1.ContainerState{},
					},
				},
			},
		},
	}

	testNodes = []runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "k8s-node-1",
				Labels: map[string]string{},
				Annotations: map[string]string{
					PHYSICAL_HOST_ANNOTATION: "abc",
					GPU_UUID_HASH_ANNOTATION: "cba",
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "127.0.0.3",
					},
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(4, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(4, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				},
			},
		},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "k8s-node-2",
				Labels: map[string]string{},
				Annotations: map[string]string{
					PHYSICAL_HOST_ANNOTATION: "def",
					GPU_UUID_HASH_ANNOTATION: "fed",
				},
			},
			Spec: corev1.NodeSpec{
				Unschedulable: false,
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{
						Type:    corev1.NodeInternalIP,
						Address: "127.0.0.4",
					},
				},
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
				Capacity: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(4, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				},
				Allocatable: corev1.ResourceList{
					corev1.ResourceCPU:    *resource.NewQuantity(4, resource.DecimalSI),
					corev1.ResourceMemory: *resource.NewQuantity(16*1024*1024*1024, resource.BinarySI),
				},
			},
		},
	}
}

func TestSyncSlurmAndK8sNodeSpecAndStatus(t *testing.T) {
	// Setup test data.
	setupTest(t)
	newScheme := runtime.NewScheme()
	_ = slonkv1.AddToScheme(newScheme)
	_ = corev1.AddToScheme(newScheme)
	runtimeObjects := append(testPods, testNodes...)

	testData := slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{
			{
				Name:     "slurm-node-1",
				State:    []string{"ALLOCATED", "RESERVED"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-1",
			},
			{
				Name:     "slurm-node-2",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-2",
			},
		},
	}

	// Start the test slurmrestd server.
	socketPath := "/tmp/test.sock"
	cleanup, err := slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Create a fake reconciler.
	testPhysicalNode := &slonkv1.PhysicalNode{}
	fakeClient := clientFake.NewClientBuilder().
		WithScheme(newScheme).
		WithRuntimeObjects(runtimeObjects...).
		WithStatusSubresource(testPhysicalNode).
		Build()
	testPhysicalNodeReconciler := &PhysicalNodeReconciler{
		Client:   fakeClient,
		Recorder: &record.FakeRecorder{},
		Scheme:   scheme.Scheme,
	}

	// Run the reconciler.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify physical nodes.
	physicalNodes := &slonkv1.PhysicalNodeList{}
	if err := fakeClient.List(context.Background(), physicalNodes); err != nil {
		t.Fatalf("Failed to list physical nodes: %v", err)
	}
	assert.Equal(t, 2, len(physicalNodes.Items))
	physicalNodeMap := make(map[string]slonkv1.PhysicalNode)
	for _, node := range physicalNodes.Items {
		physicalNodeMap[node.Name] = node
	}
	n1, ok := physicalNodeMap["cba"]
	assert.Equal(t, true, ok)
	assert.Equal(t, GoalStateUp, n1.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "slurm-node-1", n1.Status.SlurmNodeStatus.Name)
	assert.Equal(t, []string{"ALLOCATED", "RESERVED"}, n1.Status.SlurmNodeStatus.State)
	assert.Equal(t, "k8s-node-1", n1.Status.K8sNodeStatus.Name)
	assert.Equal(t, 0, len(n1.Status.SlurmNodeStatusHistory))
	n2, ok := physicalNodeMap["fed"]
	assert.True(t, ok, true)
	assert.Equal(t, GoalStateDrain, n2.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "slurm-node-2", n2.Status.SlurmNodeStatus.Name)
	assert.Equal(t, []string{"DRAIN"}, n2.Status.SlurmNodeStatus.State)
	assert.Equal(t, "k8s-node-2", n2.Status.K8sNodeStatus.Name)
	assert.Equal(t, 0, len(n2.Status.SlurmNodeStatusHistory))

	// Update data and run the reconciler again.
	cleanup()
	testData = slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{
			{
				Name:     "slurm-node-1",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-1",
			},
			{
				Name:     "slurm-node-2",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-2",
			},
		},
	}

	// Start the test slurmrestd server again.
	cleanup, err = slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Run the reconciler.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify physical nodes.
	physicalNodes = &slonkv1.PhysicalNodeList{}
	if err := fakeClient.List(context.Background(), physicalNodes); err != nil {
		t.Fatalf("Failed to list physical nodes: %v", err)
	}
	assert.Equal(t, 2, len(physicalNodes.Items))
	physicalNodeMap = make(map[string]slonkv1.PhysicalNode)
	for _, node := range physicalNodes.Items {
		physicalNodeMap[node.Name] = node
	}
	n1, ok = physicalNodeMap["cba"]
	assert.Equal(t, true, ok)
	assert.Equal(t, GoalStateDrain, n1.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "slurm-node-1", n1.Status.SlurmNodeStatus.Name)
	assert.Equal(t, []string{"DRAIN"}, n1.Status.SlurmNodeStatus.State)
	assert.Equal(t, "k8s-node-1", n1.Status.K8sNodeStatus.Name)
	assert.Equal(t, 1, len(n1.Status.SlurmNodeStatusHistory))
	n2, ok = physicalNodeMap["fed"]
	assert.Equal(t, ok, true)
	assert.Equal(t, GoalStateDrain, n2.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "slurm-node-2", n2.Status.SlurmNodeStatus.Name)
	assert.Equal(t, []string{"DRAIN"}, n2.Status.SlurmNodeStatus.State)
	assert.Equal(t, "k8s-node-2", n2.Status.K8sNodeStatus.Name)
	assert.Equal(t, 0, len(n2.Status.SlurmNodeStatusHistory))

	// Remove one slurm node.
	cleanup()
	testData = slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{
			{
				Name:     "slurm-node-2",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-2",
			},
		},
	}
	// Start the test slurmrestd server again.
	cleanup, err = slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer cleanup()

	// Run the reconciler.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify physical nodes.
	physicalNodes = &slonkv1.PhysicalNodeList{}
	if err := fakeClient.List(context.Background(), physicalNodes); err != nil {
		t.Fatalf("Failed to list physical nodes: %v", err)
	}
	assert.Equal(t, 2, len(physicalNodes.Items))
	physicalNodeMap = make(map[string]slonkv1.PhysicalNode)
	for _, node := range physicalNodes.Items {
		physicalNodeMap[node.Name] = node
	}
	n1, ok = physicalNodeMap["cba"]
	assert.Equal(t, true, ok)
	assert.Equal(t, GoalStateDrain, n1.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "", n1.Status.SlurmNodeStatus.Name)
	assert.True(t, n1.Status.SlurmNodeStatus.Removed)
	assert.Equal(t, "k8s-node-1", n1.Status.K8sNodeStatus.Name)
	assert.Equal(t, 2, len(n1.Status.SlurmNodeStatusHistory))
	n2, ok = physicalNodeMap["fed"]
	assert.Equal(t, ok, true)
	assert.Equal(t, GoalStateDrain, n2.Spec.SlurmNodeSpec.GoalState)
	assert.Equal(t, "slurm-node-2", n2.Status.SlurmNodeStatus.Name)
	assert.Equal(t, []string{"DRAIN"}, n2.Status.SlurmNodeStatus.State)
	assert.Equal(t, "k8s-node-2", n2.Status.K8sNodeStatus.Name)
	assert.Equal(t, 0, len(n2.Status.SlurmNodeStatusHistory))
}

func TestPropogateSlurmGoalStateToK8sNodeAnnotations(t *testing.T) {
	// Setup test data.
	setupTest(t)
	newScheme := runtime.NewScheme()
	_ = slonkv1.AddToScheme(newScheme)
	_ = corev1.AddToScheme(newScheme)
	runtimeObjects := append(testPods, testNodes...)

	testData := slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{
			{
				Name:     "slurm-node-1",
				State:    []string{"ALLOCATED", "RESERVED"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-1",
			},
			{
				Name:     "slurm-node-2",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-2",
			},
		},
	}

	// Start the test slurmrestd server.
	socketPath := "/tmp/test.sock"
	cleanup, err := slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer cleanup()

	// Create a fake reconciler.
	testPhysicalNode := &slonkv1.PhysicalNode{}
	fakeClient := clientFake.NewClientBuilder().
		WithScheme(newScheme).
		WithRuntimeObjects(runtimeObjects...).
		WithStatusSubresource(testPhysicalNode).
		Build()
	testPhysicalNodeReconciler := &PhysicalNodeReconciler{
		Client:   fakeClient,
		Recorder: &record.FakeRecorder{},
		Scheme:   scheme.Scheme,
	}

	// Execute sync function.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify k8s node annotations.
	corev1Nodes := &corev1.NodeList{}
	if err := fakeClient.List(context.Background(), corev1Nodes); err != nil {
		t.Fatalf("Failed to list k8s nodes: %v", err)
	}
	assert.Equal(t, 2, len(corev1Nodes.Items))
	corev1NodeMap := make(map[string]corev1.Node)
	for _, node := range corev1Nodes.Items {
		corev1NodeMap[node.Name] = node
	}
	kn1, ok := corev1NodeMap["k8s-node-1"]
	assert.Equal(t, true, ok)
	assert.Equal(t, "up", kn1.Annotations[SLURM_GOAL_STATE_ANNOTATION])
	kn2, ok := corev1NodeMap["k8s-node-2"]
	assert.Equal(t, true, ok)
	assert.Equal(t, "drain", kn2.Annotations[SLURM_GOAL_STATE_ANNOTATION])
	assert.Equal(t, "test-reason-2", kn2.Annotations[SLURM_REASON_ANNOTATION])
}

func TestPropogateSlurmGoalStateToK8sNodeTaints(t *testing.T) {
	// Setup test data.
	setupTest(t)
	newScheme := runtime.NewScheme()
	_ = slonkv1.AddToScheme(newScheme)
	_ = corev1.AddToScheme(newScheme)
	testPhysicalNode := &slonkv1.PhysicalNode{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fed",
		},
		Spec: slonkv1.PhysicalNodeSpec{
			SlurmNodeSpec: slonkv1.SlurmNodeSpec{
				GoalState: GoalStateDown,
			},
			K8sNodeSpec: slonkv1.K8sNodeSpec{
				GoalState: GoalStateUp,
			},
			Manual: true,
		},
	}
	runtimeObjects := append(testPods, testNodes...)
	runtimeObjects = append(runtimeObjects, testPhysicalNode)

	testData := slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{
			{
				Name:     "slurm-node-1",
				State:    []string{"ALLOCATED", "RESERVED"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-1",
			},
			{
				Name:     "slurm-node-2",
				State:    []string{"DRAIN"},
				Features: []string{"h100", "gpu"},
				Reason:   "test-reason-2",
			},
		},
	}

	// Start the test slurmrestd server.
	socketPath := "/tmp/test.sock"
	cleanup, err := slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}

	// Create a fake reconciler.
	fakeClient := clientFake.NewClientBuilder().
		WithScheme(newScheme).
		WithRuntimeObjects(runtimeObjects...).
		WithStatusSubresource(testPhysicalNode).
		Build()
	testPhysicalNodeReconciler := &PhysicalNodeReconciler{
		Client:   fakeClient,
		Recorder: &record.FakeRecorder{},
		Scheme:   scheme.Scheme,
	}

	// Execute sync function.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify k8s node taints.
	corev1Nodes := &corev1.NodeList{}
	if err := fakeClient.List(context.Background(), corev1Nodes); err != nil {
		t.Fatalf("Failed to list k8s nodes: %v", err)
	}
	assert.Equal(t, 2, len(corev1Nodes.Items))
	corev1NodeMap := make(map[string]corev1.Node)
	for _, node := range corev1Nodes.Items {
		corev1NodeMap[node.Name] = node
	}
	kn1, ok := corev1NodeMap["k8s-node-1"]
	assert.Equal(t, true, ok)
	assert.Equal(t, 0, len(kn1.Spec.Taints))
	kn2, ok := corev1NodeMap["k8s-node-2"]
	assert.Equal(t, true, ok)
	// assert.Equal(t, 0, len(kn2.Spec.Taints))
	assert.Equal(t, 1, len(kn2.Spec.Taints))
	assert.Equal(t, SLURM_TAINT_GOAL_STATE, kn2.Spec.Taints[0].Key)
	assert.Equal(t, "down", kn2.Spec.Taints[0].Value)
	assert.Equal(t, v1.TaintEffectNoSchedule, kn2.Spec.Taints[0].Effect)

	// Remove all CRDs, taints should still be there, but goal state should be new ("up").
	cleanup()
	testData = slurm.SlurmResponse{
		Nodes: []slurm.SlurmNode{},
	}

	// Start the test slurmrestd server.
	socketPath = "/tmp/test.sock"
	cleanup, err = slurm.StartTestSlurmRestD(socketPath, testData)
	if err != nil {
		t.Fatalf("Failed to start test server: %v", err)
	}
	defer cleanup()

	// Remove CRDs.
	physicalNodes := &slonkv1.PhysicalNodeList{}
	if err := fakeClient.List(context.Background(), physicalNodes); err != nil {
		t.Fatalf("Failed to list physical nodes: %v", err)
	}
	for _, node := range physicalNodes.Items {
		if err := fakeClient.Delete(context.Background(), &node); err != nil {
			t.Fatalf("Failed to delete physical node: %v", err)
		}
	}

	// Execute sync function.
	_, err = testPhysicalNodeReconciler.Sync(context.Background(), socketPath, false)
	assert.NoError(t, err)

	// Verify k8s node taints.
	corev1Nodes = &corev1.NodeList{}
	if err := fakeClient.List(context.Background(), corev1Nodes); err != nil {
		t.Fatalf("Failed to list k8s nodes: %v", err)
	}
	assert.Equal(t, 2, len(corev1Nodes.Items))
	corev1NodeMap = make(map[string]corev1.Node)
	for _, node := range corev1Nodes.Items {
		corev1NodeMap[node.Name] = node
	}
	kn1, ok = corev1NodeMap["k8s-node-1"]
	assert.Equal(t, true, ok)
	assert.Equal(t, 0, len(kn1.Spec.Taints))
	kn2, ok = corev1NodeMap["k8s-node-2"]
	assert.Equal(t, true, ok)
	assert.Equal(t, 1, len(kn2.Spec.Taints))
	assert.Equal(t, "up", kn2.Annotations[SLURM_GOAL_STATE_ANNOTATION])
}
