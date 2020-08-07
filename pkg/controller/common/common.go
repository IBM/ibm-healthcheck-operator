//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package common

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"
)

// GetPodNames returns the pod names of the array of pods passed in
func GetPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
}

// GetResources returns ResourceRequirements
func GetResources(res *operatorv1alpha1.Resources) *corev1.ResourceRequirements {
	var (
		requestsCPU    = resource.MustParse("50m")
		requestsMemory = resource.MustParse("64Mi")

		limitsCPU    = resource.MustParse("500m")
		limitsMemory = resource.MustParse("512Mi")
	)

	resoucesCount := 0
	ret := &corev1.ResourceRequirements{}

	requestMap := map[corev1.ResourceName]resource.Quantity{}
	if res.Requests.CPU != "" {
		resoucesCount++
		reqCPU, err := resource.ParseQuantity(res.Requests.CPU)
		if err == nil {
			requestsCPU = reqCPU
		}
		requestMap[corev1.ResourceCPU] = requestsCPU
	}
	if res.Requests.Memory != "" {
		resoucesCount++
		reqMemory, err := resource.ParseQuantity(res.Requests.Memory)
		if err == nil {
			requestsMemory = reqMemory
		}
		requestMap[corev1.ResourceMemory] = requestsMemory
	}

	limitMap := map[corev1.ResourceName]resource.Quantity{}
	if res.Limits.CPU != "" {
		resoucesCount++
		limCPU, err := resource.ParseQuantity(res.Limits.CPU)
		if err == nil {
			limitsCPU = limCPU
		}
		limitMap[corev1.ResourceCPU] = limitsCPU
	}
	if res.Limits.Memory != "" {
		resoucesCount++
		limMemory, err := resource.ParseQuantity(res.Limits.Memory)
		if err == nil {
			limitsMemory = limMemory
		}
		limitMap[corev1.ResourceMemory] = limitsMemory
	}

	if resoucesCount == 0 {
		requestMap[corev1.ResourceCPU] = requestsCPU
		requestMap[corev1.ResourceMemory] = requestsMemory
		limitMap[corev1.ResourceCPU] = limitsCPU
		limitMap[corev1.ResourceMemory] = limitsMemory
	}

	ret.Requests = requestMap
	ret.Limits = limitMap

	return ret
}
