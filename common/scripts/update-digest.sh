#!/bin/bash
#
# Copyright 2020 IBM Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

#VERSION=$(cat version/version.go  | grep "Version =" | awk -F "\"" '{print $2}')
VERSION=3.9.0
CSV_FILE="deploy/olm-catalog/ibm-healthcheck-operator/${VERSION}/ibm-healthcheck-operator.v${VERSION}.clusterserviceversion.yaml"

docker pull hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/system-healthcheck-service:3.8.0
HEALTHCHECK_DIGEST=$(docker images --digests  | grep -E "hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/system-healthcheck-service.*3.8.0" | awk '{print $3}')
docker pull hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/icp-memcached:3.8.1
MEMCACHED_DIGEST=$(docker images --digests  | grep -E "hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/icp-memcached.*3.8.1" | awk '{print $3}')
docker pull hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/must-gather:4.5.3
MUSTGATHER_JOB_DIGEST=$(docker images --digests  | grep -E "hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/must-gather.*4.5.3" | awk '{print $3}')
docker pull hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/must-gather-service:1.1.0
MUSTGATHER_SERVICE_DIGEST=$(docker images --digests  | grep -E "hyc-cloud-private-integration-docker-local.artifactory.swg-devops.com/ibmcom/must-gather-service.*1.1.0" | awk '{print $3}')

sed -i "/SYSTEM_HEALTHCHECK_SERVICE_IMAGE/{n;s/sha256.*/$HEALTHCHECK_DIGEST\"/;}" "${CSV_FILE}"
sed -i "/ICP_MEMCACHED_IMAGE/{n;s/sha256.*/$MEMCACHED_DIGEST\"/;}" "${CSV_FILE}"
sed -i "/MUST_GATHER_IMAGE/{n;s/sha256.*/$MUSTGATHER_JOB_DIGEST\"/;}" "${CSV_FILE}"
sed -i "/MUST_GATHER_SERVICE_IMAGE/{n;s/sha256.*/$MUSTGATHER_SERVICE_DIGEST\"/;}" "${CSV_FILE}"
