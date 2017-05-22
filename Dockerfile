FROM alpine:3.5

# Install jsonnet first
RUN apk update \
&& apk add \
     git \
     build-base \
     ca-certificates \
     openssl \
&& rm -rf /var/cache/apk/*

WORKDIR /tmp
RUN git clone https://github.com/google/jsonnet.git \
  && cd jsonnet \
  && git reset --hard v0.9.3 \
  && make jsonnet \
  && cp jsonnet /usr/local/bin \
  && cd /tmp \
  && rm -rf jsonnet

RUN mkdir -p /app/internal

# Copy data into project directory.
WORKDIR /app
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/core.libsonnet \
&& wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/util.libsonnet \
&& cd internal \
&& wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/assert.libsonnet \
&& wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/base.libsonnet \
&& wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/meta.libsonnet \
&& cd /app

# Put the (pre-built by the Makefile) app in place
COPY /ksonnet-playground /

EXPOSE 8080
CMD /ksonnet-playground
