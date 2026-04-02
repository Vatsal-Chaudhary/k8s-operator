package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KafkaScalerSpec defines the desired state of KafkaScaler.
type KafkaScalerSpec struct {
	Topic           string `json:"topic"`
	ConsumerGroup   string `json:"consumerGroup"`
	DeploymentName  string `json:"deploymentName"`
	TargetNamespace string `json:"targetNamespace"`

	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`

	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	LagPerReplica   int64    `json:"lagPerReplica"`
	CooldownSeconds int      `json:"cooldownSeconds"`
	KafkaBrokers    []string `json:"kafkaBrokers"`
}

// KafkaScalerStatus defines the observed state of KafkaScaler.
type KafkaScalerStatus struct {
	CurrentReplicas int32        `json:"currentReplicas"`
	CurrentLag      int64        `json:"currentLag"`
	LastScaleTime   *metav1.Time `json:"lastScaleTime,omitempty"`

	// Conditions represent the latest available observations of the resource state.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="CurrentLag",type="integer",JSONPath=".status.currentLag"
// +kubebuilder:printcolumn:name="CurrentReplicas",type="integer",JSONPath=".status.currentReplicas"
// +kubebuilder:printcolumn:name="LastScaleTime",type="date",JSONPath=".status.lastScaleTime"

// KafkaScaler is the Schema for the kafkascalers API.
type KafkaScaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaScalerSpec   `json:"spec,omitempty"`
	Status KafkaScalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KafkaScalerList contains a list of KafkaScaler.
type KafkaScalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaScaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaScaler{}, &KafkaScalerList{})
}
