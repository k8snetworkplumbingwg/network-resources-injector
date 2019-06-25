#!/bin/bash

# Always exit on errors.
set -e

# Set our known directories and parameters.
BASE_DIR=$(cd $(dirname $0)/..; pwd)
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
    PARAM=`echo $1 | awk -F= '{print $1}'`
    VALUE=`echo $1 | awk -F= '{print $2}'`
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

kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/service.yaml
export NAMESPACE
cat ${BASE_DIR}/deployments/webhook.yaml | \
	${BASE_DIR}/scripts/webhook-patch-ca-bundle.sh | \
	sed -e "s|\${NAMESPACE}|${NAMESPACE}|g" | \
	kubectl -n ${NAMESPACE} create -f -

kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/auth.yaml
kubectl -n ${NAMESPACE} create -f ${BASE_DIR}/deployments/server.yaml
