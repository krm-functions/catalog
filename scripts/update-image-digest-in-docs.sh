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

IMAGE=ghcr.io/krm-functions/helm-upgrader
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/helm-upgrader.*@).*#\1$DIGEST#" docs/*.md

IMAGE=ghcr.io/krm-functions/render-helm-chart
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" examples/render-helm-chart/Kptfile
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" examples/digester/Kptfile

IMAGE=ghcr.io/krm-functions/source-helm-chart
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/source-helm-chart.*@).*#\1$DIGEST#" docs/*.md

IMAGE=ghcr.io/krm-functions/apply-setters
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "Using digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/apply-setters.*@).*#\1$DIGEST#" docs/*.md
