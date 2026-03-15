ARG ARG_FROM
ARG ARG_BUILDER_IMAGE

FROM ${ARG_BUILDER_IMAGE} AS build

ARG ARG_HELM_VERSION
ARG TARGETPLATFORM=linux/amd64

RUN echo "Building for ${TARGETPLATFORM} using Helm ${ARG_HELM_VERSION}"
ENV TARGETPLATFORM=${TARGETPLATFORM}
ENV ARG_OS="${TARGETPLATFORM%/*}"
ENV ARG_ARCH="${TARGETPLATFORM#*/}"

RUN apk update && apk add curl && curl -fsSL -o /helm-${ARG_HELM_VERSION}-${ARG_OS}-${ARG_ARCH}.tar.gz https://get.helm.sh/helm-${ARG_HELM_VERSION}-${ARG_OS}-${ARG_ARCH}.tar.gz && \
     tar -zxvf /helm-${ARG_HELM_VERSION}-${ARG_OS}-${ARG_ARCH}.tar.gz && \
     mv ./${ARG_OS}-${ARG_ARCH}/helm /usr/local/bin/helm

FROM ${ARG_FROM}

# Helm tools add-on
COPY --from=build /usr/local/bin/helm /usr/local/bin/helm
ENV PATH=/usr/local/bin:$PATH
ENV HELM_PATH_CACHE=/var/cache
ENV HELM_CONFIG_HOME=/tmp/helm/config
ENV HELM_CACHE_HOME=/tmp/helm/cache

RUN apk add skopeo

USER nobody:nobody
ENV HOME=/
