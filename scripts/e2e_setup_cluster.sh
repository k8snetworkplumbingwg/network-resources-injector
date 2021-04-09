#!/bin/bash
set -eo pipefail
# Create CA cert and key which will be used as root CA for Kubernetes.
# we also create 2 yaml artifacts to enable K8 API server flag
# --admission-control-config. MutatatingAdmissionWebhook controller
# will read its credentials stored in a kube config file attached to this flag.
# For simplicity sake, we use the pre-generated CA cert and key as the
# credentials for NRI.

root="$(dirname "$0")/../"
export PATH="${PATH}:${root:?}/bin"
RETRY_MAX=10
INTERVAL=10
TIMEOUT=300
APP_NAME="network-resources-injector"
APP_DOCKER_TAG="${APP_NAME}:latest"
K8_ADDITIONS_PATH="${root}/scripts/control-plane-additions"
TMP_DIR="${root}/test/tmp"
MULTUS_DAEMONSET_URL="https://raw.githubusercontent.com/intel/multus-cni/master/images/multus-daemonset.yml"
MULTUS_NAME="multus"
CNIS_DAEMONSET_URL="https://raw.githubusercontent.com/intel/multus-cni/master/e2e/cni-install.yml"
CNIS_NAME="cni-plugins"
TEST_NAMESPACE="default"

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

create_cluster() {
  [ -z "${mount_dir}" ] && echo "### no mount directory set" && exit 1
  # deploy cluster with kind
  cat <<EOF | kind create cluster --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
- |
  kind: ClusterConfiguration
  apiServer:
    extraArgs:
      admission-control-config-file: /etc/kubernetes/pki/ac.yaml
nodes:
  - role: control-plane
    extraMounts:
    - hostPath: "${mount_dir:?}"
      containerPath: "/etc/kubernetes/pki"
  - role: worker
EOF
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

check_requirements() {
  for cmd in docker kind openssl kubectl base64; do
    if ! command -v "$cmd" &> /dev/null; then
      echo "$cmd is not available"
      exit 1
    fi
  done
}

create_foo_network() {
cat <<EOF | kubectl apply -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: example.com/foo
  name: foo-network
  namespace: ${TEST_NAMESPACE}
spec:
  config: |
    {
      "cniVersion": "0.3.0",
      "name": "foo-network",
      "type": "loopback"
    }
EOF
}

echo "## checking requirements"
check_requirements
# generate K8 API server CA key/cert and supporting files for mTLS with NRI
echo "## generating K8 api flags files"
generate_k8_api_data
echo "## start Kind cluster with precreated CA key/cert"
create_cluster
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
retry kubectl create -f "${root}/deployments/server.yaml"
retry kubectl -n kube-system wait --for=condition=ready -l app="${APP_NAME}" pod --timeout=300s
echo "## create network foo"
create_foo_network
sleep 5
echo "## starting kube proxy"
nohup kubectl proxy -p=8001 > /dev/null 2>&1 &
proxy_pid=$!
sleep 1
echo "## adding capacity of 4 example.com/foo to kind-worker node"
curl -g --retry ${RETRY_MAX} --retry-delay ${INTERVAL} --connect-timeout ${TIMEOUT}  --header "Content-Type: application/json-patch+json" \
  --request PATCH --data '[{"op": "add", "path": "/status/capacity/example.com~1foo", "value": "100"}]' \
  http://127.0.0.1:8001/api/v1/nodes/kind-worker/status > /dev/null
echo "## killing kube proxy"
kill $proxy_pid
