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

package v1alpha1

// image defines the desired image repository, tag and imagepullpolicy
type image struct {
	// image repository, default is empty
	Repository string `json:"repository"`
	// image tag, default is empty
	Tag string `json:"tag"`
	// image pull policy, default is IfNotPresent
	PullPolicy string `json:"pullPolicy,omitempty"`
}
