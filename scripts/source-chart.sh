#! /usr/bin/env bash

RENDER_HELM_CHART_RESOURCE=$1
CHART_FILE=$2 # Optional

# Uses the jq-based yq from devbox.json (-r: raw strings, -y: YAML output)
name=$(yq -r '.metadata.name' $RENDER_HELM_CHART_RESOURCE)
annotations=$(yq -y '.metadata.annotations' $RENDER_HELM_CHART_RESOURCE)
chartname=$(yq -r '.helmCharts | .[0].chartArgs.name' $RENDER_HELM_CHART_RESOURCE)
repo=$(yq -r '.helmCharts | .[0].chartArgs.repo' $RENDER_HELM_CHART_RESOURCE)
version=$(yq -r '.helmCharts | .[0].chartArgs.version' $RENDER_HELM_CHART_RESOURCE)
chartargs=$(yq -y '.helmCharts | .[0].chartArgs' $RENDER_HELM_CHART_RESOURCE)
templopts=$(yq -y '.helmCharts | .[0].templateOptions' $RENDER_HELM_CHART_RESOURCE)

if [[ -z "$CHART_FILE" ]]; then
        outname="$chartname-$version.tgz" # FIXME, common format, but not guaranteed
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
