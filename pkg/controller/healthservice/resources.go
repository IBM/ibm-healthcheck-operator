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

package healthservice

import (
	"regexp"
	"strconv"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func (r *ReconcileHealthService) getResources(res *operatorv1alpha1.Resources) *corev1.ResourceRequirements {
	var (
		requestsCPU    = resource.NewMilliQuantity(50, resource.DecimalSI)      // 50m,
		requestsMemory = resource.NewQuantity(64*1024*1024, resource.BinarySI)  // 64Mi
		limitsCPU      = resource.NewMilliQuantity(500, resource.DecimalSI)     // 500m
		limitsMemory   = resource.NewQuantity(512*1024*1024, resource.BinarySI) // 512Mi
	)

	resoucesCount := 0
	ret := &corev1.ResourceRequirements{}

	requestMap := map[corev1.ResourceName]resource.Quantity{}
	if res.Requests.CPU != "" {
		resoucesCount++
		reqCPU, err := GetDigits(res.Requests.CPU)
		if err == nil {
			requestsCPU = resource.NewMilliQuantity(reqCPU, resource.DecimalSI)
		}
		requestMap[corev1.ResourceCPU] = *requestsCPU
	}
	if res.Requests.Memory != "" {
		resoucesCount++
		reqMemory, err := GetDigits(res.Requests.Memory)
		if err == nil {
			requestsMemory = resource.NewQuantity(reqMemory*1024*1024, resource.BinarySI)
		}
		requestMap[corev1.ResourceMemory] = *requestsMemory
	}

	limitMap := map[corev1.ResourceName]resource.Quantity{}
	if res.Limits.CPU != "" {
		resoucesCount++
		limCPU, err := GetDigits(res.Limits.CPU)
		if err == nil {
			limitsCPU = resource.NewMilliQuantity(limCPU, resource.DecimalSI)
		}
		limitMap[corev1.ResourceCPU] = *limitsCPU
	}
	if res.Limits.Memory != "" {
		resoucesCount++
		limMemory, err := GetDigits(res.Limits.Memory)
		if err == nil {
			limitsMemory = resource.NewQuantity(limMemory*1024*1024, resource.BinarySI)
		}
		limitMap[corev1.ResourceMemory] = *limitsMemory
	}

	if resoucesCount == 0 {
		requestMap[corev1.ResourceCPU] = *requestsCPU
		requestMap[corev1.ResourceMemory] = *requestsMemory
		limitMap[corev1.ResourceCPU] = *limitsCPU
		limitMap[corev1.ResourceMemory] = *limitsMemory
	}

	ret.Requests = requestMap
	ret.Limits = limitMap

	return ret
}

var digitsRegexp = regexp.MustCompile("([0-9]*)")

func GetDigits(s string) (int64, error) {
	strDigits := digitsRegexp.FindStringSubmatch(s)[1]
	digits, err := strconv.ParseInt(strDigits, 10, 64)
	return digits, err
}
