set -e

ORG_PATH="github.com/intel"
REPO_PATH="${ORG_PATH}/network-resources-injector"

if [ ! -h .gopath/src/${REPO_PATH} ]; then
	mkdir -p .gopath/src/${ORG_PATH}
	ln -s ../../../.. .gopath/src/${REPO_PATH} || exit 255
fi

export GOPATH=${PWD}/.gopath
export GOBIN=${PWD}/bin
export CGO_ENABLED=0
export GO15VENDOREXPERIMENT=1

go install -ldflags "-s -w" -buildmode=pie -tags no_openssl "$@" ${REPO_PATH}/cmd/installer
go install -ldflags "-s -w" -buildmode=pie -tags no_openssl "$@" ${REPO_PATH}/cmd/webhook
