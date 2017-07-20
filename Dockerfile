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
  && git reset --hard v0.9.4 \
  && make jsonnet \
  && cp jsonnet /usr/local/bin \
  && cd /tmp \
  && rm -rf jsonnet

RUN mkdir /app

# Copy data into project directory.
WORKDIR /app
COPY /ext/ksonnet-lib/ksonnet.alpha.1/ ./ksonnet.alpha.1/
COPY /ext/ksonnet-lib/ksonnet.beta.1/ ./ksonnet.beta.1/
COPY /ext/ksonnet-lib/ksonnet.beta.2/ ./ksonnet.beta.2/

# Put the (pre-built by the Makefile) app in place
COPY /ksonnet-playground /

EXPOSE 8080
CMD /ksonnet-playground
