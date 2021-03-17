#!/bin/bash
set -eo pipefail
# Create CA cert and key which will be used as root CA for Kubernetes.
# we also create 2 yaml artifacts to enable K8 API server flag
# --admission-control-config. MutatatingAdmissionWebhook controller
# will read its credentials stored in a kube config file attached to this flag.
# For simplicity sake, we use the pre-generated CA cert and key as the
# credentials for NRI.

here="$(dirname "$(readlink --canonicalize "${BASH_SOURCE[0]}")")"
root="$(readlink --canonicalize "$here/..")"
RETRY_MAX=10
INTERVAL=30
TIMEOUT=300
APP_NAME="network-resources-injector"
APP_DOCKER_TAG="${APP_NAME}:latest"
K8_ADDITIONS_PATH="${root}/scripts/control-plane-additions"
TMP_DIR="/tmp"
MULTUS_DAEMONSET_URL="https://raw.githubusercontent.com/intel/multus-cni/master/images/multus-daemonset.yml"
MULTUS_NAME="multus"
CNIS_DAEMONSET_URL="https://raw.githubusercontent.com/intel/multus-cni/master/e2e/cni-install.yml"
CNIS_NAME="cni-plugins"
TEST_NAMESPACE="default"
# array with the KinD workers
KIND_WORKER_NAMES=( kind-worker kind-worker2 )

# create cluster CA and API server admission configuration
# to force API server and NRI authentication.
# CA cert & key along with supporting yamls will be mounted to control plane
# path /etc/kubernetes/pki. Kubeadm will utilise generated CA cert/key as root
# Kubernetes CA. Cert passed to NRI will be signed by this CA.
generate_k8_api_data() {
  mkdir -p "${TMP_DIR}"
  mount_dir="$(mktemp -q -p "${TMP_DIR}" -d -t nri-e2e-k8-api-pki-XXXXXXXX)"
  echo "### creating K8 CA cert & private key"
  openssl req \
   -nodes \
   -subj "/C=IE/ST=None/L=None/O=None/CN=kubernetes" \
   -new -x509 \
   -days 1 \
   -keyout "${mount_dir:?}/ca.key" \
   -out "${mount_dir}/ca.crt" > /dev/null 2>&1
  echo "### add admission config for K8 API server"
  # add kube config file for NRI
  cp "${K8_ADDITIONS_PATH}/ac.yaml" "${K8_ADDITIONS_PATH}/mutatingkubeconfig.yaml" \
    "${mount_dir}/"
  echo "### add cert & key data to kube config template"
  cert_data="$(base64 -w 0 < "${mount_dir}/ca.crt")"
  key_data="$(base64 -w 0 < "${mount_dir}/ca.key")"
  sed -i -e "s/CERT/${cert_data}/" -e "s/KEY/${key_data}/" \
    "${mount_dir}/mutatingkubeconfig.yaml"
}

retry() {
  local status=0
  local retries=${RETRY_MAX:=5}
  local delay=${INTERVAL:=5}
  local to=${TIMEOUT:=20}
  cmd="$*"

  while [ $retries -gt 0 ]
  do
    status=0
    timeout $to bash -c "echo $cmd && $cmd" || status=$?
    if [ $status -eq 0 ]; then
      break;
    fi
    echo "Exit code: '$status'. Sleeping '$delay' seconds before retrying"
    sleep $delay
    let retries--
  done
  return $status
}

create_cluster() {
  [ -z "${mount_dir}" ] && echo "### no mount directory set" && exit 1

  # create list of worker nodes
  workers="$(for i in "${KIND_WORKER_NAMES[@]}"; do echo "  - role: worker"; done)"

  # create KinD configuration file
  exec 3<> "${PWD}"/kindConfig.yaml

    # Let's print Kind configuration to file to fd 3
    echo "kind: Cluster" >&3
    echo "apiVersion: kind.x-k8s.io/v1alpha4" >&3
    echo "featureGates:" >&3
    echo "  DownwardAPIHugePages: true" >&3
    echo "kubeadmConfigPatches:" >&3
    echo "- |" >&3
    echo "  kind: ClusterConfiguration" >&3
    echo "  apiServer:" >&3
    echo "    extraArgs:" >&3
    echo "      admission-control-config-file: /etc/kubernetes/pki/ac.yaml" >&3
    echo "nodes:" >&3
    echo "  - role: control-plane" >&3
    echo "    extraMounts:" >&3
    echo "    - hostPath: \"${mount_dir:?}\"" >&3
    echo "      containerPath: \"/etc/kubernetes/pki\"" >&3
    echo "${workers}" >&3

  # Close fd 3
  exec 3>&-

  # deploy cluster with kind
  retry kind delete cluster && kind create cluster --config="${PWD}"/kindConfig.yaml

  rm "${PWD}"/kindConfig.yaml
}

check_requirements() {
  for cmd in docker kind openssl kubectl base64; do
    if ! command -v "$cmd" &> /dev/null; then
      echo "$cmd is not available"
      exit 1
    fi
  done
}

patch_kind_node() {
  echo "## Adding capacity of example.com/foo to $1 node"
  curl -g --retry ${RETRY_MAX} --retry-delay ${INTERVAL} --connect-timeout ${TIMEOUT}  --header "Content-Type: application/json-patch+json" \
    --request PATCH --data '[{"op": "add", "path": "/status/capacity/example.com~1foo", "value": "100"}]' \
    http://127.0.0.1:8001/api/v1/nodes/"$1"/status > /dev/null

  curl -g --retry ${RETRY_MAX} --retry-delay ${INTERVAL} --connect-timeout ${TIMEOUT}  --header "Content-Type: application/json-patch+json" \
    --request PATCH --data '[{"op": "add", "path": "/status/capacity/example.com~1boo", "value": "100"}]' \
    http://127.0.0.1:8001/api/v1/nodes/"$1"/status > /dev/null
}

echo "## checking requirements"
check_requirements
# generate K8 API server CA key/cert and supporting files for mTLS with NRI
echo "## generating K8 api flags files"
generate_k8_api_data
echo "## start Kind cluster with precreated CA key/cert"
create_cluster
echo "## remove taints from master node"
kubectl taint nodes kind-control-plane node-role.kubernetes.io/master:NoSchedule-
echo "## build NRI"
retry docker build -t "${APP_DOCKER_TAG}" "${root}"
echo "## load NRI image into Kind"
kind load docker-image "${APP_DOCKER_TAG}"
echo "## export kube config for utilising locally"
kind export kubeconfig
echo "## install coreDNS"
kubectl -n kube-system wait --for=condition=available deploy/coredns --timeout=300s
echo "## install multus"
retry kubectl create -f "${MULTUS_DAEMONSET_URL}"
retry kubectl -n kube-system wait --for=condition=ready -l name="${MULTUS_NAME}" pod --timeout=300s
echo "## install CNIs"
retry kubectl create -f "${CNIS_DAEMONSET_URL}"
retry kubectl -n kube-system wait --for=condition=ready -l name="${CNIS_NAME}" pod --timeout=300s
echo "## install NRI"
retry kubectl create -f "${root}/deployments/auth.yaml"
retry kubectl create -f "${root}/deployments/server_huge.yaml"
retry kubectl -n kube-system wait --for=condition=ready -l app="${APP_NAME}" pod --timeout=300s
sleep 5
echo "## starting kube proxy"
nohup kubectl proxy -p=8001 > /dev/null 2>&1 &
proxy_pid=$!
sleep 1

echo "## adding capacity of 4 example.com/foo to kind-worker node"
for (( i = 0; i < "${#KIND_WORKER_NAMES[@]}"; i++ )); do
  patch_kind_node "${KIND_WORKER_NAMES[${i}]}" || true
done

echo "## killing kube proxy"
kill $proxy_pid
