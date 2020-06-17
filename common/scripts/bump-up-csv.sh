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

if [ -z "${1}" ] || [ -z "${2}" ]; then
    echo "Usage: $0 [OPERATOR_NAME] [NEW_CSV_VERSION]"
    exit 1
fi

if [ ! -f "$(command -v yq 2> /dev/null)" ]; then
    echo "[ERROR] yq command not found"
    exit 1
fi

OPERATOR_NAME=${1}
NEW_CSV_VERSION=${2}

DEPLOY_DIR=${DEPLOY_DIR:-deploy}
BUNDLE_DIR=${BUNDLE_DIR:-deploy/olm-catalog/${OPERATOR_NAME}}
LAST_CSV_DIR=$(find "${BUNDLE_DIR}" -type d -maxdepth 1 | sort | tail -1)
LAST_CSV_VERSION=$(basename "${LAST_CSV_DIR}")
NEW_CSV_DIR=${LAST_CSV_DIR//${LAST_CSV_VERSION}/${NEW_CSV_VERSION}}

# PREVIOUS_CSV_DIR=$(find "${BUNDLE_DIR}" -type d -maxdepth 1 | sort | tail -2 | head -1)
# PREVIOUS_CSV_VERSION=$(basename "${PREVIOUS_CSV_DIR}")

if [ "${LAST_CSV_VERSION}" == "${NEW_CSV_VERSION}" ]; then
    echo "Last CSV version is already at ${NEW_CSV_VERSION}"
    exit 1
fi
# echo "[INFO] Bumping up CSV version from ${LAST_CSV_VERSION} to ${NEW_CSV_VERSION}"
# cp -rfv "${LAST_CSV_DIR}" "${NEW_CSV_DIR}"
# OLD_CSV_FILE=$(find "${NEW_CSV_DIR}" -type f -name '*.clusterserviceversion.yaml' | head -1)
# NEW_CSV_FILE=${OLD_CSV_FILE//${LAST_CSV_VERSION}.clusterserviceversion.yaml/${NEW_CSV_VERSION}.clusterserviceversion.yaml}
# if [ -f "${OLD_CSV_FILE}" ]; then
#     mv -v "${OLD_CSV_FILE}" "${NEW_CSV_FILE}"
# fi
echo "[INFO] Generating CSV version from ${LAST_CSV_VERSION} to ${NEW_CSV_VERSION}"
operator-sdk generate k8s
operator-sdk generate crds
operator-sdk generate csv --make-manifests=false --csv-version "${NEW_CSV_VERSION}" --update-crds --from-version "${LAST_CSV_VERSION}"
LAST_CRD_FILE=$(find "${LAST_CSV_DIR}" -type f -name '*_crd.yaml' | head -1)
NEW_CRD_FILE=$(find "${NEW_CSV_DIR}" -type f -name '*_crd.yaml' | head -1)
LAST_CSV_FILE=$(find "${LAST_CSV_DIR}" -type f -name '*.clusterserviceversion.yaml' | head -1)
NEW_CSV_FILE=$(find "${NEW_CSV_DIR}" -type f -name '*.clusterserviceversion.yaml' | head -1)

echo "[INFO] Updating ${NEW_CRD_FILE}"
add_labels=$(yq r "${LAST_CRD_FILE}" metadata.labels)
yq w -i "${NEW_CRD_FILE}" "metadata.labels" "${add_labels}"

echo "[INFO] Updating ${NEW_CSV_FILE}"
spec_CRD=$(yq r "${LAST_CSV_FILE}" spec.customresourcedefinitions)
yq w -i "${NEW_CSV_FILE}" "spec.customresourcedefinitions" "${spec_CRD}"
containers=$(yq r "${LAST_CSV_FILE}" spec.install)
yq w -i "${NEW_CSV_FILE}" "spec.install" "${containers}"
# REPLACES_VERSION=$(yq r "${NEW_CSV_FILE}" "metadata.name")
# sed -e "s|name: ${OPERATOR_NAME}\(.*\)${LAST_CSV_VERSION}|name: ${OPERATOR_NAME}\1${NEW_CSV_VERSION}|" -i "${NEW_CSV_FILE}"
# sed -e "s|olm.skipRange: \(.*\)${LAST_CSV_VERSION}\(.*\)|olm.skipRange: \1${NEW_CSV_VERSION}\2|" -i "${NEW_CSV_FILE}"
# sed -e "s|image: \(.*\)${OPERATOR_NAME}\(.*\)|image: \1${OPERATOR_NAME}:latest|" -i "${NEW_CSV_FILE}"
# sed -e "s|containerImage: \(.*\)${OPERATOR_NAME}\(.*\)|containerImage: \1${OPERATOR_NAME}:latest|" -i "${NEW_CSV_FILE}"
# sed -e "s|replaces: ${OPERATOR_NAME}\(.*\)${PREVIOUS_CSV_VERSION}|replaces: ${REPLACES_VERSION}|" -i "${NEW_CSV_FILE}"
# sed -e "s|version: ${LAST_CSV_VERSION}|version: ${NEW_CSV_VERSION}|" -i "${NEW_CSV_FILE}"

PACKAGE_YAML=${BUNDLE_DIR}/${OPERATOR_NAME}.package.yaml
echo "[INFO] Updating ${PACKAGE_YAML}"
NEW_VERSION=$(yq r "${NEW_CSV_FILE}" "metadata.name")
yq w -i "${PACKAGE_YAML}" "channels.(name==dev).currentCSV" "${NEW_VERSION}" 

echo "[INFO] Updating ${DEPLOY_DIR}/operator.yaml"
sed -e "s|image: \(.*\)${OPERATOR_NAME}\(.*\)|image: \1${OPERATOR_NAME}:latest|" -i "${DEPLOY_DIR}/operator.yaml"

if [ -f ".osdk-scorecard.yaml" ]; then
    echo "[INFO] Updating .osdk-scorecard.yaml"
    sed -e "s|${LAST_CSV_VERSION}|${NEW_CSV_VERSION}|g" -i .osdk-scorecard.yaml
fi

if [ -d "helm-charts" ]; then
    CHART_FILE=$(find helm-charts -type f -name 'Chart.yaml' | head -1)
    if [ -f "${CHART_FILE}" ]; then
        echo "[INFO] Updating ${CHART_FILE}"
        yq w -i "${CHART_FILE}" "version" "${NEW_CSV_VERSION}"
    fi

    MANIFEST_FILE=$(find helm-charts -type f -name 'manifest.yaml' | head -1)
    if [ -f "${MANIFEST_FILE}" ]; then
        echo "[INFO] Updating ${MANIFEST_FILE}"
        sed -e "s|${OPERATOR_NAME}:${LAST_CSV_VERSION}|${OPERATOR_NAME}:${NEW_CSV_VERSION}|" -i "${MANIFEST_FILE}"
    fi
fi

VERSION_GO="version/version.go"
if [ -f "${VERSION_GO}" ]; then
    echo "[INFO] Updating ${VERSION_GO}"
    sed -e "s|Version\(.*\)${LAST_CSV_VERSION}\(.*\)|Version\1${NEW_CSV_VERSION}\2|" -i "${VERSION_GO}"
fi