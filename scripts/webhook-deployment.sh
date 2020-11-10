#!/bin/bash

# Copyright (c) 2020 Intel Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Always exit on errors.
set -e

# Set our known directories and parameters.
BASE_DIR="$(cd "$(dirname "$0")"/..; pwd)"
NAMESPACE="kube-system"

# Give help text for parameters.
function usage()
{
    echo -e "./webhook-deployment.sh"
    echo -e "\t-h --help"
    echo -e "\t--namespace=${NAMESPACE}"
}


# Parse parameters given as arguments to this script.
while [ "$1" != "" ]; do
    PARAM="$(echo "$1" | awk -F= '{print $1}')"
    VALUE="$(echo "$1" | awk -F= '{print $2}')"
    case $PARAM in
        -h | --help)
            usage
            exit
            ;;
        --namespace)
            NAMESPACE=$VALUE
            ;;
        *)
            echo "ERROR: unknown parameter \"$PARAM\""
            usage
            exit 1
            ;;
    esac
    shift
done

kubectl -n "${NAMESPACE}" create -f "${BASE_DIR}/deployments/service.yaml"
export NAMESPACE
cat "${BASE_DIR}/deployments/webhook.yaml" | \
	"${BASE_DIR}/scripts/webhook-patch-ca-bundle.sh" | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n "${NAMESPACE}" create -f -

kubectl -n "${NAMESPACE}" create -f "${BASE_DIR}/deployments/auth.yaml"
kubectl -n "${NAMESPACE}" create -f "${BASE_DIR}/deployments/server.yaml"
