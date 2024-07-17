#!/bin/bash

set -ex

TMP_SCHEMAS=tmp-schemas
mkdir -p "$TMP_SCHEMAS/kubernetes-json-schema" "$TMP_SCHEMAS/CRDs-catalog"

git clone --depth 1 --single-branch --branch master https://github.com/yannh/kubernetes-json-schema.git "$TMP_SCHEMAS/kubernetes-json-schema"
git clone --depth 1 --single-branch --branch master https://github.com/datreeio/CRDs-catalog.git        "$TMP_SCHEMAS/CRDs-catalog"

# Build schema bundle
SCHEMAS_BUNDLE=schema-bundle
mkdir -p "$SCHEMAS_BUNDLE"
cp -r "$TMP_SCHEMAS/kubernetes-json-schema/master-standalone-strict" "$SCHEMAS_BUNDLE/"
cp -r "$TMP_SCHEMAS/CRDs-catalog" "$SCHEMAS_BUNDLE/"

rm -rf "$TMP_SCHEMAS"
