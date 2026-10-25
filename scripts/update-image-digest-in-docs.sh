#!/usr/bin/env bash

set -e

TAG=$1

SCRIPT=$(readlink -f $0)
SCRIPTPATH=$(dirname $SCRIPT)

if [ -z "$TAG" ]; then
        # SHA=`git rev-parse --short HEAD`
        # TAG="$SHA"
        TAG=latest
        echo "No image tag specified, using: $TAG"
fi

IMAGE=ghcr.io/krm-functions/helm-upgrader
DIGEST=$(crane digest $IMAGE:$TAG)
echo "helm-upgrader digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/helm-upgrader.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/helm-upgrader.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/render-helm-chart
DIGEST=$(crane digest $IMAGE:$TAG)
echo "render-helm-chart digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/render-helm-chart.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/source-helm-chart
DIGEST=$(crane digest $IMAGE:$TAG)
echo "source-helm-chart digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/source-helm-chart.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/source-helm-chart.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/apply-setters
DIGEST=$(crane digest $IMAGE:$TAG)
echo "apply-setters digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/apply-setters.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/apply-setters.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/digester
DIGEST=$(crane digest $IMAGE:$TAG)
echo "digester digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/digester.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/digester.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/kubeconform
DIGEST=$(crane digest $IMAGE:$TAG)
echo "kubeconform digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/kubeconform.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/kubeconform.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/package-compositor
DIGEST=$(crane digest $IMAGE:$TAG)
echo "package-compositor digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/package-compositor.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/package-compositor.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/set-annotations
DIGEST=$(crane digest $IMAGE:$TAG)
echo "set-annotations digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/set-annotations.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/set-annotations.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/set-labels
DIGEST=$(crane digest $IMAGE:$TAG)
echo "set-labels digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/set-labels.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/set-labels.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/remove-local-config-resources
DIGEST=$(crane digest $IMAGE:$TAG)
echo "remove-local-config-resources digest: $DIGEST"
sed -i -E "s#(.*?ghcr.io/krm-functions/remove-local-config-resources.*@).*#\1$DIGEST#" docs/*.md
sed -i -E "s#(.*?ghcr.io/krm-functions/remove-local-config-resources.*@).*#\1$DIGEST#" Makefile.test
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST

IMAGE=ghcr.io/krm-functions/gatekeeper-set-enforcement-action
DIGEST=$(crane digest $IMAGE:$TAG)
echo "gatekeeper-set-enforcement-action digest: $DIGEST"
$SCRIPTPATH/update-catalog.sh $IMAGE $DIGEST
