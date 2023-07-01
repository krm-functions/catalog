#! /bin/bash

RENDER_HELM_CHART_RESOURCE=$1

chartname=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i mikefarah/yq:4.24.5 '.helmCharts | .[0].chartArgs.name' -)
repo=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i mikefarah/yq:4.24.5 '.helmCharts | .[0].chartArgs.repo' -)
version=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i mikefarah/yq:4.24.5 '.helmCharts | .[0].chartArgs.version' -)
templopts=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i mikefarah/yq:4.24.5 '.helmCharts | .[0].templateOptions' -)

outname="$chartname-$version.tgz"
echo "Fetching $chartname/$version@$repo --> $outname" 1>&2

if [[ $repo =~ ^oci:// ]]; then
    helm pull $repo/$chartname --version $version
else
    helm pull $chartname --repo $repo --version $version
fi

shasum=$(sha256sum $outname | cut -d' ' -f1)

cat <<EOF
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: $chartname
  annotations:
    experimental.helm.sh/chart-sum: "sha256:$shasum"
    experimental.helm.sh/chart-name: "$chartname"
    experimental.helm.sh/chart-repo: "$repo"
    experimental.helm.sh/chart-version: "$version"
spec:
EOF

echo "  templateOptions:"
echo "$templopts" | sed 's/^/    /'
echo "  chart:"
base64 $outname | sed 's/^/    /'

rm $outname
