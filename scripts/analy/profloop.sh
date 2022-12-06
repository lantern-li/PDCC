#!/bin/bash

ip=$1
path=$2

if [ -z "$ip" ]; then
    echo "please specify ip:port: [usage] heap.sh ip:port"
    exit
fi

if [ -z "${path}" ]; then
    path=/tmp/prof
fi
mkdir -p $path
while :
do
    fileNo=$(date "+%Y-%m-%d_%H:%M:%S")
    echo $fileNo
    curl -sK -v  http://"${ip}"/debug/pprof/heap > ${path}/heap"${fileNo}".prof
    curl -sK -v  http://"${ip}"/debug/pprof/goroutine > ${path}/goroutine"${fileNo}".prof
    sleep 120
done