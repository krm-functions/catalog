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
echo "helm-upgrader digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/helm-upgrader.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/helm-upgrader.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/render-helm-chart
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "render-helm-chart digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" examples/render-helm-chart/Kptfile
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" examples/digester/Kptfile
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/source-helm-chart
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "source-helm-chart digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/source-helm-chart.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/source-helm-chart.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/apply-setters
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "apply-setters digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/apply-setters.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/apply-setters.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/digester
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "digester digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/digester.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/digester.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/kubeconform
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "kubeconform digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/kubeconform.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/kubeconform.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/package-compositor
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "package-compositor digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/package-compositor.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/package-compositor.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/set-annotations
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "set-annotations digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/set-annotations.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/set-annotations.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/set-labels
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "set-labels digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/set-labels.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/set-labels.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/remove-local-config-resources
DIGEST=$($SCRIPTPATH/../scripts/skopeo.sh inspect docker://$IMAGE:$TAG | jq -r .Digest)
echo "remove-local-config-resources digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/remove-local-config-resources.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/remove-local-config-resources.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST
