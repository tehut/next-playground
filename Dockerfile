FROM golang:1.8

EXPOSE 8080

# Create go directory, copy project source in.
RUN mkdir -p /go/src/ksonnet-playground/internal
ADD main.go /go/src/ksonnet-playground

# Copy data into project directory.
WORKDIR /go/src/ksonnet-playground
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/core.libsonnet
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/util.libsonnet
WORKDIR /go/src/ksonnet-playground/internal
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/assert.libsonnet
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/base.libsonnet
RUN wget https://raw.githubusercontent.com/ksonnet/ksonnet-lib/master/kube/internal/meta.libsonnet

WORKDIR /go/src/ksonnet-playground
RUN go build main.go

CMD ./main
