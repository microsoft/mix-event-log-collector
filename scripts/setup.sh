#!/bin/bash

TAG=
ALPINE=`(cat /etc/os-release || echo "" | grep "ID=alpine")`
if [ -n "$ALPINE" ]; then
    TAG=" -tags musl "
fi

# 3rd-party tools
echo "Getting 3rd party tools..."
go get -d github.com/swaggo/swag/cmd/swag
echo "done"
echo

# sync module dependcies
echo "Getting vendor module dependencies..."
go mod init nuance.xaas-logging.event-log-collector
go mod tidy
go mod vendor
echo "done"
echo

build_targets=("fetcher" "processor" "writer" "client")
for t in "${build_targets[@]}"
do
    echo "building target: $t"
    go build $TAG -o ./bin/event-log-$t cmd/event-log-$t/main.go 
    echo "build complete"
    echo
done
