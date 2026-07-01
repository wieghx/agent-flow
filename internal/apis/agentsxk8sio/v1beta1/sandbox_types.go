// Copyright 2025 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package v1beta1 provides types and constants for the Sandbox CRD.
// This package re-exports types from sigs.k8s.io/agent-sandbox/api/v1beta1.
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"
)

type ConditionType = sandboxv1beta1.ConditionType

const (
	SandboxConditionSuspended              = sandboxv1beta1.SandboxConditionSuspended
	SandboxReasonSuspendedPodTerminated    = sandboxv1beta1.SandboxReasonSuspendedPodTerminated
	SandboxReasonSuspendedPodNotTerminated = sandboxv1beta1.SandboxReasonSuspendedPodNotTerminated
	SandboxConditionReady                  = sandboxv1beta1.SandboxConditionReady
	SandboxReasonDependenciesReady         = sandboxv1beta1.SandboxReasonDependenciesReady
	SandboxReasonDependenciesNotReady      = sandboxv1beta1.SandboxReasonDependenciesNotReady
	SandboxReasonSuspended                 = sandboxv1beta1.SandboxReasonSuspended
	SandboxConditionFinished               = sandboxv1beta1.SandboxConditionFinished
	SandboxReasonPodSucceeded              = sandboxv1beta1.SandboxReasonPodSucceeded
	SandboxReasonPodFailed                 = sandboxv1beta1.SandboxReasonPodFailed
	SandboxReasonExpired                   = sandboxv1beta1.SandboxReasonExpired
	SandboxPodNameAnnotation               = sandboxv1beta1.SandboxPodNameAnnotation
	SandboxTemplateRefAnnotation           = sandboxv1beta1.SandboxTemplateRefAnnotation
	SandboxLaunchTypeLabel                 = sandboxv1beta1.SandboxLaunchTypeLabel
	SandboxLaunchTypeCold                  = sandboxv1beta1.SandboxLaunchTypeCold
	SandboxLaunchTypeWarm                  = sandboxv1beta1.SandboxLaunchTypeWarm
	SandboxPodTemplateHashLabel            = sandboxv1beta1.SandboxPodTemplateHashLabel
	SandboxPropagatedLabelsAnnotation      = sandboxv1beta1.SandboxPropagatedLabelsAnnotation
	SandboxPropagatedAnnotationsAnnotation = sandboxv1beta1.SandboxPropagatedAnnotationsAnnotation
	SandboxAdoptableLabel                  = sandboxv1beta1.SandboxAdoptableLabel
	SandboxWarmPoolLabel                   = sandboxv1beta1.SandboxWarmPoolLabel
)

type PodMetadata = sandboxv1beta1.PodMetadata
type EmbeddedObjectMetadata = sandboxv1beta1.EmbeddedObjectMetadata
type PodTemplate = sandboxv1beta1.PodTemplate
type PersistentVolumeClaimTemplate = sandboxv1beta1.PersistentVolumeClaimTemplate
type SandboxOperatingMode = sandboxv1beta1.SandboxOperatingMode

const (
	SandboxOperatingModeRunning   = sandboxv1beta1.SandboxOperatingModeRunning
	SandboxOperatingModeSuspended = sandboxv1beta1.SandboxOperatingModeSuspended
)

type SandboxSpec = sandboxv1beta1.SandboxSpec
type Lifecycle = sandboxv1beta1.Lifecycle
type ShutdownPolicy = sandboxv1beta1.ShutdownPolicy

const (
	ShutdownPolicyDelete = sandboxv1beta1.ShutdownPolicyDelete
	ShutdownPolicyRetain = sandboxv1beta1.ShutdownPolicyRetain
)

type Sandbox = sandboxv1beta1.Sandbox
type SandboxList = sandboxv1beta1.SandboxList
type SandboxStatus = sandboxv1beta1.SandboxStatus

func AddToScheme(s *runtime.Scheme) error {
	return sandboxv1beta1.AddToScheme(s)
}
