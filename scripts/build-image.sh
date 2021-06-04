#!/bin/bash

# Copyright (c) 2019 Intel Corporation
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

# Note: Execute only from the package root directory or top-level Makefile!

set -e

# set proxy args
BUILD_ARGS=()
[ -n "$http_proxy"  ] && BUILD_ARGS+=("--build-arg http_proxy=$http_proxy")
[ -n "$HTTP_PROXY"  ] && BUILD_ARGS+=("--build-arg HTTP_PROXY=$HTTP_PROXY")
[ -n "$https_proxy" ] && BUILD_ARGS+=("--build-arg https_proxy=$https_proxy")
[ -n "$HTTPS_PROXY" ] && BUILD_ARGS+=("--build-arg HTTPS_PROXY=$HTTPS_PROXY")
[ -n "$no_proxy"    ] && BUILD_ARGS+=("--build-arg no_proxy=$no_proxy")
[ -n "$NO_PROXY"    ] && BUILD_ARGS+=("--build-arg NO_PROXY=$NO_PROXY")
[ -n "$HTTP_PROXY"  ] && BUILD_ARGS+=("--build-arg HTTP_PROXY=$HTTP_PROXY")
[ -n "$HTTPS_PROXY" ] && BUILD_ARGS+=("--build-arg HTTPS_PROXY=$HTTPS_PROXY")
[ -n "$NO_PROXY"    ] && BUILD_ARGS+=("--build-arg NO_PROXY=$NO_PROXY")

# build admission controller Docker image
docker build ${BUILD_ARGS[@]} -f Dockerfile -t network-resources-injector .
