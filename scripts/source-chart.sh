#! /bin/bash

RENDER_HELM_CHART_RESOURCE=$1
CHART_FILE=$2   # Optional

YQ=mikefarah/yq:4.24.5

name=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.metadata.name' -)
annotations=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.metadata.annotations' -)
chartname=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.helmCharts | .[0].chartArgs.name' -)
repo=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.helmCharts | .[0].chartArgs.repo' -)
version=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.helmCharts | .[0].chartArgs.version' -)
chartargs=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.helmCharts | .[0].chartArgs' -)
templopts=$(cat $RENDER_HELM_CHART_RESOURCE | docker run --rm -i $YQ '.helmCharts | .[0].templateOptions' -)



if [[ -z "$CHART_FILE" ]]; then
    outname="$chartname-$version.tgz"  # FIXME, common format, but not guaranteed
    echo "Fetching $chartname/$version@$repo --> $outname" 1>&2
    if [[ $repo =~ ^oci:// ]]; then
	helm pull $repo/$chartname --version $version
    else
	helm pull $chartname --repo $repo --version $version
    fi
else
    outname=$CHART_FILE
    echo "Using existing chart file $outname" 1>&2
fi

shasum=$(sha256sum $outname | cut -d' ' -f1)

cat <<EOF
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: $name
  annotations:
    experimental.helm.sh/chart-sum: "sha256:$shasum"
    $annotations
helmCharts:
EOF

echo "- chartArgs:"
echo "$chartargs" | sed 's/^/    /'
echo "  templateOptions:"
echo "$templopts" | sed 's/^/    /'
echo "  chart: |"
base64 $outname | sed 's/^/    /'

if [[ -z "$CHART_FILE" ]]; then
    rm $outname
fi
