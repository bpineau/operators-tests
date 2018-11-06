/*
Copyright 2018 Datadog Inc..

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
)

// MonitorSpec defines the desired state of Monitor
type MonitorSpec struct {
	Type    string         `json:"type,omitempty"`
	Query   string         `json:"query,omitempty"`
	Message string         `json:"message,omitempty"`
	Name    string         `json:"name,omitempty"`
	Tags    []string       `json:"tags,omitempty"`
	Options *OptionsConfig `json:"options,omitempty"`
}

// MonitorStatus defines the observed state of Monitor
type MonitorStatus struct {
	Phase string `json:"phase,omitempty"`
	ID    int64  `json:"id,omitempty"`
}

// OptionsConfig defines some of the possible options values
type OptionsConfig struct {
	NotifyAudit       *bool   `json:"notify_audit,omitempty"`
	Locked            *bool   `json:"locked,omitempty"`
	NoDataTimeFrame   *int64  `json:"no_data_timeframe,omitempty"`
	NewHostDelay      *int64  `json:"new_host_delay,omitempty"`
	RequireFullWindow *bool   `json:"require_full_window,omitempty"`
	NotifyNoData      *bool   `json:"notify_no_data,omitempty"`
	TimeoutH          *int64  `json:"timeout_h,omitempty"`
	RenotifyInterval  *int64  `json:"renotify_interval,omitempty"`
	EscalationMessage *string `json:"escalation_message,omitempty"`
	IncludeTags       *bool   `json:"include_tags,omitempty"`
	// TODO: silenced, thresholds, threshold_windows, evaluation_delay
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Monitor is the Schema for the monitors API
// +k8s:openapi-gen=true
type Monitor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MonitorSpec   `json:"spec,omitempty"`
	Status MonitorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MonitorList contains a list of Monitor
type MonitorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Monitor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Monitor{}, &MonitorList{})
}
