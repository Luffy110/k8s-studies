#!/bin/sh
GOOS=${GOOS:-linux}
GOARCH=${GOARCH:-amd64}
GOBINARY=${GOBINARY:-go}
LDFLAGS="-extldflags -static"
BINDIR=".release/bin"
PACKAGES=`go list ./cmd/...`
export CGO_ENABLED=0

echo "go mod vendor"

go mod vendor

echo "Generating files..."
for i in ${PACKAGES}; do
  NAME=`basename $i`
  echo "Building $i..."
  GOOS=${GOOS} GOARCH=${GOARCH} ${GOBINARY} build -o ${BINDIR}/dynamic-webhook -ldflags "${LDFLAGS}" "${i}"
  echo "Installed .release/bin/dynamic-webhook"
done