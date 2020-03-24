/*
Copyright 2018 The Kubernetes Authors.

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

package node

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/core"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeClass defines a class of container runtime supported in the cluster.
// The RuntimeClass is used to determine which container runtime is used to run
// all containers in a pod. RuntimeClasses are (currently) manually defined by a
// user or cluster provisioner, and referenced in the PodSpec. The Kubelet is
// responsible for resolving the RuntimeClassName reference before running the
// pod.  For more details, see
// https://git.k8s.io/enhancements/keps/sig-node/runtime-class.md
type RuntimeClass struct {
	metav1.TypeMeta
	// +optional
	metav1.ObjectMeta

	// Handler specifies the underlying runtime and configuration that the CRI
	// implementation will use to handle pods of this class. The possible values
	// are specific to the node & CRI configuration.  It is assumed that all
	// handlers are available on every node, and handlers of the same name are
	// equivalent on every node.
	// For example, a handler called "runc" might specify that the runc OCI
	// runtime (using native Linux containers) will be used to run the containers
	// in a pod.
	// The Handler must conform to the DNS Label (RFC 1123) requirements, and is
	// immutable.
	Handler string

	// Overhead represents the resource overhead associated with running a pod for a
	// given RuntimeClass. For more details, see
	// https://git.k8s.io/enhancements/keps/sig-node/20190226-pod-overhead.md
	// This field is alpha-level as of Kubernetes v1.16, and is only honored by servers
	// that enable the PodOverhead feature.
	// +optional
	Overhead *Overhead

	// Scheduling holds the scheduling constraints to ensure that pods running
	// with this RuntimeClass are scheduled to nodes that support it.
	// If scheduling is nil, this RuntimeClass is assumed to be supported by all
	// nodes.
	// +optional
	Scheduling *Scheduling
}

// Overhead structure represents the resource overhead associated with running a pod.
type Overhead struct {
	//  PodFixed represents the fixed resource overhead associated with running a pod.
	// +optional
	PodFixed core.ResourceList
}

// Scheduling specifies the scheduling constraints for nodes supporting a
// RuntimeClass.
type Scheduling struct {
	// nodeSelector lists labels that must be present on nodes that support this
	// RuntimeClass. Pods using this RuntimeClass can only be scheduled to a
	// node matched by this selector. The RuntimeClass nodeSelector is merged
	// with a pod's existing nodeSelector. Any conflicts will cause the pod to
	// be rejected in admission.
	// +optional
	NodeSelector map[string]string

	// tolerations are appended (excluding duplicates) to pods running with this
	// RuntimeClass during admission, effectively unioning the set of nodes
	// tolerated by the pod and the RuntimeClass.
	// +optional
	Tolerations []core.Toleration
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RuntimeClassList is a list of RuntimeClass objects.
type RuntimeClassList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// Items is a list of schema objects.
	Items []RuntimeClass
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExecutionHook defines a specific action that should be taken with timeout
type ExecutionHook struct {
	metav1.TypeMeta
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta

	// Specification of the ExecutionHook
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec ExecutionHookSpec

	// Status represents the current information about a hook
	// +optional
	Status ExecutionHookStatus
}

// ExecutionHook defines a specific action that should be taken with timeout
type ExecutionHookSpec struct {
	// Command to execute for a particular trigger
	HookHandler core.Handler

	// How long kubelet should wait for the hook to complete execution
	// before giving up. Kubelet needs to succeed or fail within this
	// timeout period, regardless of the retries. If not set, kubelet should
	// set a default timeout.
	// +optional
	TimeoutSeconds *int64
}

type ExecutionHookConditionType string

// These are valid conditions of ExecutionHooks
const (
	// Hook command is waiting to be triggered
	ExecutionHookPending ExecutionHookConditionType = "HookPending"
	// Hook command is being executed
	ExecutionHookExecuting ExecutionHookConditionType = "HookExecuting"
	// Hook command is completed
	ExecutionHookCompleted ExecutionHookConditionType = "HookCompleted"
)

// ExecutionHookCondition represents the current condition of ExecutionHook
type ExecutionHookCondition struct {
	Type   ExecutionHookConditionType
	Status core.ConditionStatus
	// +optional
	LastProbeTime *metav1.Time
	// +optional
	LastTransitionTime *metav1.Time
	// +optional
	Reason *string
	// +optional
	Message *string
}

type ExecutionHookStatus struct {
	// If not set, it is nil, indicating Action has not started
	// If set, it means Action has started at the specified time
	// +optional
	Timestamp *int64

	// ActionSucceed is set to true when the action is executed in the container successfully.
	// It will be set to false if the action cannot be executed successfully after ActionTimeoutSeconds passes.
	// +optional
	Succeed *bool

	// The last error encountered when executing the action. The hook controller might update this field each time
	// it retries the execution.
	// +optional
	Error *HookError
	// +optional
	Conditions []ExecutionHookCondition
}

type ErrorType string

const (
	// The execution hook times out
	Timeout ErrorType = "Timeout"

	// The execution hook fails with an error
	Error ErrorType = "Error"
)

type HookError struct {
	// Type of the error
	// This is required
	ErrorType ErrorType

	// Error message
	// +optional
	Message *string

	// More detailed reason why error happens
	// +optional
	Reason *string

	// It indicates when the error occurred
	// +optional
	Timestamp *int64
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExecutionHookList is a list of ExecutionHook objects.
type ExecutionHookList struct {
	metav1.TypeMeta
	// +optional
	metav1.ListMeta

	// Items is a list of schema objects.
	Items []ExecutionHook
}
