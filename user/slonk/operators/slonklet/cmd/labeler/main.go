package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	slonkv1 "your-org.com/slonklet/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// Util to label physical nodes.
func main() {
	// Create a new client.
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Printf("Error getting kubeconfig: %s\n", err)
		os.Exit(1)
	}
	k8sClient, err := client.New(cfg, client.Options{})
	if err != nil {
		fmt.Printf("Error creating client: %s\n", err)
		os.Exit(1)
	}

	// List CRs.
	scheme := runtime.NewScheme()
	groupVersionKind := schema.GroupVersionKind{
		Group:   "slonk.your-org.com",
		Version: "v1",
		Kind:    "PhysicalNode",
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fmt.Printf("Error adding clientgoscheme to scheme: %s\n", err)
	}
	if err := slonkv1.AddToScheme(scheme); err != nil {
		fmt.Printf("Error adding slonkv1 to scheme: %s\n", err)
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(groupVersionKind)
	if err := k8sClient.List(context.Background(), list, client.InNamespace("slurm")); err != nil {
		panic(err)
	}
	fmt.Printf("list length: %d\n", len(list.Items))
	var currentPhysicalNodeMap = make(map[string]*slonkv1.PhysicalNode)
	for _, item := range list.Items {
		// fmt.Printf("item: %v\n", item)
		var currentPhysicalNode slonkv1.PhysicalNode
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &currentPhysicalNode); err != nil {
			panic(err)
		}
		currentPhysicalNodeMap[currentPhysicalNode.Name] = &currentPhysicalNode
	}

	//rawPhysicalNodeListBytes, err := os.ReadFile("/Users/yiranwang/Code/physicalnodes_20240507_good.out")
	rawPhysicalNodeListBytes, err := os.ReadFile("/Users/yiranwang/Code/physicalnodes_20240505.out")
	if err != nil {
		panic(err)
	}

	var physicalNodeList slonkv1.PhysicalNodeList
	if err := yaml.Unmarshal(rawPhysicalNodeListBytes, &physicalNodeList); err != nil {
		panic(err)
	}
	physicalNodeMap := make(map[string]*slonkv1.PhysicalNode)
	for i := range physicalNodeList.Items {
		physicalNode := physicalNodeList.Items[i]
		physicalNodeMap[physicalNode.Name] = &physicalNode
	}

	foundCount := 0
	notFoundCount := 0

	rawK8sNodeListFile, err := os.Open("/Users/yiranwang/Code/sdc_hunt_20240506.txt")
	if err != nil {
		fmt.Println(err)
	}
	defer rawK8sNodeListFile.Close()
	fileScanner := bufio.NewScanner(rawK8sNodeListFile)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		k8sNodeName := strings.Trim(fileScanner.Text(), " ")
		found := false
		var currentPhysicalNode *slonkv1.PhysicalNode
		var ok bool
		for _, physicalNode := range physicalNodeMap {
			if physicalNode.Status.K8sNodeStatus.Name == k8sNodeName {
				found = true
				currentPhysicalNode, ok = currentPhysicalNodeMap[physicalNode.Name]
				if !ok {
					fmt.Printf("currentPhysicalNode not found: %s\n", physicalNode.Name)
				}
			} else {
				for _, history := range physicalNode.Status.K8sNodeStatusHistory {
					if history.Name == k8sNodeName {
						found = true
						currentPhysicalNode, ok = currentPhysicalNodeMap[physicalNode.Name]
						if !ok {
							fmt.Printf("currentPhysicalNode not found: %s\n", physicalNode.Name)
						}
						break
					}
				}
			}
			if found {
				foundCount++
				// fmt.Printf("k8sNode: %s, currentPhysicalNode: %s\n", k8sNodeName, currentPhysicalNode.Name)
				newPhysicalNode := currentPhysicalNode.DeepCopy()
				if newPhysicalNode.Labels == nil {
					newPhysicalNode.Labels = make(map[string]string)
				}
				if newPhysicalNode.Labels["sdc-hunt-20240507"] == "true" {
					// fmt.Printf("k8sNode: %s already has label\n", k8sNodeName)
				} else {
					newPhysicalNode.Labels["sdc-hunt-20240507"] = "true"
					u := &unstructured.Unstructured{}
					u.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(newPhysicalNode)
					if err != nil {
						panic(err)
					}
					if err := k8sClient.Update(context.Background(), u); err != nil {
						fmt.Printf("Error updating physical node: %s\n", err)
					} else {
						fmt.Printf("Updated physical node: %s\n", newPhysicalNode.Name)
					}
				}
				break
			}
		}
		if !found {
			notFoundCount++
			fmt.Printf("k8sNodeName: %s not found\n", k8sNodeName)
		}
	}

	fmt.Printf("foundCount: %d\n", foundCount)
	fmt.Printf("notFoundCount: %d\n", notFoundCount)
}
