#! /bin/bash

set -e

TAG=$1

SCRIPT=$(readlink -f $0)
SCRIPTPATH=`dirname $SCRIPT`

if [ -z "$TAG" ]; then
    SHA=`git rev-parse --short HEAD`
    TAG="$SHA"
    echo "No image tag specified, using HEAD: $TAG"
fi

IMAGE=ghcr.io/michaelvl/krm-helm-upgrader
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/michaelvl/krm-.*@).*#\1$DIGEST#" docs/helm-upgrader.md

IMAGE=ghcr.io/michaelvl/krm-render-helm-chart
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/michaelvl/krm-.*@).*#\1$DIGEST#" docs/render-helm-chart.md
