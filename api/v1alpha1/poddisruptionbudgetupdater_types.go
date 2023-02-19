/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Settings applied to PodDisruptionBudget
type SettingsSpec struct {
	MinAvailable   *intstr.IntOrString `json:"minAvailable,omitempty"`
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// Default settings to apply to PodDisruptionBudget
type DefaultSpec struct {
	Settings SettingsSpec `json:"settings"`
}

// Periods where non-default settings are to be applied to PodDisruptionBudget
type PeriodSpec struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Rules for time determed non-default settings to apply to PodDisruptonBudget
type RulesSpec struct {
	Periods  []PeriodSpec `json:"periods"`
	Settings SettingsSpec `json:"settings"`
}

// PodDisruptionBudgetUpdaterSpec defines the desired state of PodDisruptionBudgetUpdater
type PodDisruptionBudgetUpdaterSpec struct {
	// PodDisruptionBudget update configuration and rules
	PodDisruptionBudgets []string    `json:"podDisruptionBudgets"`
	Default              DefaultSpec `json:"default"`
	Rules                []RulesSpec `json:"rules"`
}

type PodDisruptionBudgetStatus struct {
	LastUpdated string `json:"lastUpdated,omitempty"`
	// If the managed PodDisruptionBudget was found
	Found  bool   `json:"found"`
	Status string `json:"status,omitempty"`
}

// PodDisruptionBudgetUpdaterStatus defines the observed state of PodDisruptionBudgetUpdater
type PodDisruptionBudgetUpdaterStatus struct {
	// Current status of managed PodDisruptionBudget
	PodDisruptionBudget []PodDisruptionBudgetStatus `json:"podDisruptionBudget,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PodDisruptionBudgetUpdater is the Schema for the poddisruptionbudgetupdaters API
type PodDisruptionBudgetUpdater struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodDisruptionBudgetUpdaterSpec   `json:"spec,omitempty"`
	Status PodDisruptionBudgetUpdaterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PodDisruptionBudgetUpdaterList contains a list of PodDisruptionBudgetUpdater
type PodDisruptionBudgetUpdaterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PodDisruptionBudgetUpdater `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PodDisruptionBudgetUpdater{}, &PodDisruptionBudgetUpdaterList{})
}
