# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# busybox:v1.36.0
FROM busybox@sha256:acaddd9ed544f7baf3373064064a51250b14cfe3ec604d65765a53da5958e5f5 as release

ARG PRODUCT_NAME=nomad
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
# TARGETARCH and TARGETOS are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL maintainer="Nomad Team <nomad@hashicorp.com>"
LABEL version=${PRODUCT_VERSION}
LABEL revision=${PRODUCT_REVISION}

COPY dist/$TARGETOS/$TARGETARCH/nomad /bin/
COPY ./scripts/docker-entrypoint.sh /

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]
