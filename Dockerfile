# syntax=docker/dockerfile:1.7

FROM golang:1.22-bookworm

ARG PROTOC_GEN_GO_VERSION=v1.34.2
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.5.1

RUN apt-get update \
    && apt-get install -y --no-install-recommends protobuf-compiler ca-certificates git \
    && rm -rf /var/lib/apt/lists/*

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION} \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}

ENV PATH="/go/bin:${PATH}"

WORKDIR /workspace

COPY go.mod ./
RUN go mod download

CMD ["bash"]
