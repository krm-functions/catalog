#!/bin/bash

#set -x
set -e

IMAGE=quay.io/skopeo/stable:latest
CMD="docker run --rm  ${IMAGE}"
${CMD} "$@"
