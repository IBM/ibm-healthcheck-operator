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

import "strings"

// DefaultImageRegistry ...
const DefaultImageRegistry = "quay.io/opencloudio"

// DefaultImageTag ...
const DefaultImageTag = "latest"

// HealthServiceImageName ...
const HealthServiceImageName = "system-healthcheck-service"

// MemcachedImageName ...
const MemcachedImageName = "icp-memcached"

// GetOperandImage returns the operand image name, either <imageRepository>@<SHA> or <imageRepository>:<tag>
func GetOperandImage(imageRepository, imageName, imageTag, imageTagOrSHA string) string {
	var image string

	// a SHA value looks like "quay.io/opencloudio/system-healthcheck-service@sha256:e5c0d2831527705d3689515afec755291b23c2a365b5825d5fadb5d48227afdb".
	// a tag value looks like "quay.io/opencloudio/system-healthcheck-service:3.5.0".

	// For 3.3 CR, imageRepository is like quay.io/opencloudio/system-healthcheck-service
	if len(imageTagOrSHA) > 0 && len(imageTag) > 0 {
		image = imageRepository + "@" + imageTagOrSHA
	} else if len(imageTagOrSHA) > 0 && len(imageTag) == 0 {
		// For 3.4 CR, imageRepository is like quay.io/opencloudio
		if strings.HasPrefix(imageTagOrSHA, "sha256:") {
			image = imageRepository + "/" + imageName + "@" + imageTagOrSHA
		} else {
			image = imageRepository + "/" + imageName + ":" + imageTagOrSHA
		}
	} else {
		image = DefaultImageRegistry + "/" + imageName + ":" + DefaultImageTag
	}

	return image
}
