ARG ARG_BASE_IMAGE_DISTROLESS
FROM ${ARG_BASE_IMAGE_DISTROLESS}

COPY schema-bundle /schema-bundle
ENV KUBECONFORM_SCHEMA_LOCATIONS="/schema-bundle/,/schema-bundle/CRDs-catalog/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json"

