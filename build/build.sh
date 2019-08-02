#!/bin/bash

export LANG="en_US.UTF-8"
WORKROOT="$(cd $(dirname "$0") && cd ../ && pwd || false)"
export GO111MODULE=on
export GOPROXY=https://goproxy.io
cd $WORKROOT

function build() {
    OUTPUT=$WORKROOT/output
    rm -rf $OUTPUT && mkdir $OUTPUT

    # build clustercontroller and cluster shim
    OUTPUT_BIN=$OUTPUT/bin
    go build -o $OUTPUT_BIN/clustercontroller ./cmd/clustercontroller && \
        go build -o $OUTPUT_BIN/k8s_cluster_shim ./cmd/k8s_cluster_shim && \
        go build -o $OUTPUT_BIN/k3s_cluster_shim ./cmd/k3s_cluster_shim && \
        echo "build done"
}

function test() {
    #go list ./pkg/... | grep -v "pkg/generated" | grep -v "pkg/apis" | xargs -n1 go test -cover
    go test ./pkg/... -coverprofile cover.out
    totalcover=`go tool cover -func cover.out | grep total | awk '{print $3}'`
    rm cover.out
    echo "total coverage: $totalcover"
}

function usage() {
    echo >&2 "Usage:"
    echo >&2 "  $0 build"
    echo >&2 "  $0 test"
    exit 1
}

cmd="${1:-}"
if [[ ! $cmd ]]; then
    usage
fi
shift

case "${cmd}" in
    build)
        build
        ;;
    test)
        test
        ;;
    *)
        usage
        ;;
esac
