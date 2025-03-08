#!/usr/bin/bash

set -e

IMAGE=$1
DIGEST=$2

jq --arg image "$IMAGE" --arg digest "$DIGEST" '.functions = [.functions[] | if (.image == $image) then (.digest = $digest) else . end]' catalog.json | tee catalog-tmp.json
mv catalog-tmp.json catalog.json
