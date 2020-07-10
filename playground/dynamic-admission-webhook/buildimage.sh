#!/bin/bash

if [ $# -ne 2 ]
then
    echo "usage:$0 docker_user tag"
    exit
fi

DODKCER_USER=$1
TAG=$2
docker build --no-cache -t $DODKCER_USER/dynamic-admission-webhook:$TAG .
echo "$DODKCER_USER/dynamic-admission-webhook:$TAG"
rm -rf release

# if you have docker hub account then you can use this command to push this image to your remote repo of on the docker hub.
# docker push $DODKCER_USER/dynamic-admission-webhook:$TAG