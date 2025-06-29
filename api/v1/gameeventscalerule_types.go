package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EventScalingRule defines the scaling parameters for a specific game event type.
type EventScalingRule struct {
	EventType          string `json:"eventType"`          // Type of the game event (e.g., "MassPvPEvent")
	TargetMicroservice string `json:"targetMicroservice"` // Name of the Deployment to scale
	DesiredReplicas    int32  `json:"desiredReplicas"`    // Target replicas during the event
	PreScaleMinutes    int32  `json:"preScaleMinutes"`    // Minutes before event start to scale up
	PostScaleMinutes   int32  `json:"postScaleMinutes"`   // Minutes after event end to scale down
	DefaultReplicas    int32  `json:"defaultReplicas"`    // Default replicas after event ends
}

// GameEventScaleRuleSpec defines the desired state of GameEventScaleRule
type GameEventScaleRuleSpec struct {
	EventEndpointURL string             `json:"eventEndpointURL"` // URL to your game event API endpoint
	PollingInterval  string             `json:"pollingInterval"`  // How often to poll the event endpoint (e.g., "1m", "30s")
	Rules            []EventScalingRule `json:"rules"`            // List of scaling rules for different event types
}

// GameEventScaleRuleStatus defines the observed state of GameEventScaleRule
type GameEventScaleRuleStatus struct {
	LastEventCheckTime *metav1.Time        `json:"lastEventCheckTime,omitempty"` // Timestamp of the last successful event endpoint check
	ActiveScales       []ActiveScaleStatus `json:"activeScales,omitempty"`       // Currently active scaling operations
}

// ActiveScaleStatus represents the status of an ongoing scaling operation
type ActiveScaleStatus struct {
	EventType          string       `json:"eventType"`
	TargetMicroservice string       `json:"targetMicroservice"`
	ScaledToReplicas   int32        `json:"scaledToReplicas"`
	ScaleTriggerTime   *metav1.Time `json:"scaleTriggerTime"`
	EventEndTime       *metav1.Time `json:"eventEndTime"` // When the event is expected to finish
	Status             string       `json:"status"`       // "ScalingUp", "Active", "ScalingDown", "Completed"
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GameEventScaleRule is the Schema for the gameeventscalerules API
type GameEventScaleRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GameEventScaleRuleSpec   `json:"spec,omitempty"`
	Status GameEventScaleRuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GameEventScaleRuleList contains a list of GameEventScaleRule
type GameEventScaleRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GameEventScaleRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GameEventScaleRule{}, &GameEventScaleRuleList{})
}
