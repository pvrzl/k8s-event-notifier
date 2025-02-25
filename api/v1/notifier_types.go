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
	EventTypes []string `json:"eventTypes"`

	// Event reasons to filter notifications (e.g., ImagePullFailed, CrashLoopBackOff)
	// +optional
	EventReasons []string `json:"eventReasons,omitempty"`

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

	// Enable detailed logging of events in Slack messages
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
