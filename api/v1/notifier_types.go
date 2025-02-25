/*
Copyright 2025.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Channel string

const (
	Slack Channel = "slack"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NotifierSpec defines the desired state of Notifier.
type NotifierSpec struct {
	// Channel to use
	// +kubebuilder:validation:Enum=slack
	Channel Channel `json:"channel"`

	// Namespaces to monitor for events
	// +kubebuilder:validation:MinItems=1
	Namespaces []string `json:"namespaces"`

	// Event types to notify on (e.g., Warning, Normal)
	// +kubebuilder:validation:MinItems=1
	// full list can be found at: https://github.com/kubernetes/kubernetes/blob/b11d0fbdd58394a62622787b38e98a620df82750/pkg/apis/core/types.go#L4670
	EventTypes []string `json:"eventTypes"`

	// List of specific event reasons to filter notifications (e.g., Created, Started, Failed, Killing).
	// These are well-defined Kubernetes event reasons.
	// full list can be found at: https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/events/event.go
	// +optional
	EventReasons []string `json:"eventReasons,omitempty"`

	// List of substrings to match within event messages for filtering notifications.
	// Useful for capturing issues like ImagePullFailed or CrashLoopBackOff,
	// which are typically found in event messages rather than standard event reasons.
	// If not specified, the event will not be filtered by this criteria.
	// +optional
	MessageContains []string `json:"messageContains,omitempty"`

	// List of Kubernetes object types to monitor (e.g., Pod, Node, Deployment).
	// If not specified, events for all object types will be monitored.
	// full list can be found at: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	EventObjectTypes []string `json:"eventObjectTypes,omitempty"`

	// Target webhook URL
	// +kubebuilder:validation:Pattern=`^https?://.+`
	Webhook string `json:"webhook"`

	// Default settings to apply if not provided
	// +optional
	DefaultSettings *NotifierDefaults `json:"defaultSettings,omitempty"`
}

// NotifierDefaults defines optional default settings for notification formatting
type NotifierDefaults struct {
	// Prefix for messages (e.g., "[K8s Alert]")
	// +optional
	MessagePrefix string `json:"messagePrefix,omitempty"`

	// Enable detailed logging of events
	// +optional
	EnableVerbose bool `json:"enableVerbose,omitempty"`
}

// NotifierStatus defines the observed state of Notifier.
type NotifierStatus struct {
	// Current observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Last event processed timestamp
	// +optional
	LastEventTime *metav1.Time `json:"lastEventTime,omitempty"`

	// List of last processed events for debugging
	// +optional
	RecentEvents []string `json:"recentEvents,omitempty"`

	// Status message
	// +optional
	StatusMessage string `json:"statusMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Notifier is the Schema for the notifiers API.
type Notifier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NotifierSpec   `json:"spec,omitempty"`
	Status NotifierStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NotifierList contains a list of Notifier.
type NotifierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Notifier `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Notifier{}, &NotifierList{})
}
