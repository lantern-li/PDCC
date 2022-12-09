#!/bin/bash

IP=$1
MONITOR_PORT=$2
PPROF_PORT=$2
PATH=$2

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
    FILE_NO=$(date "+%Y-%m-%d_%H:%M:%S")
    echo "$FILE_NO"
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/heap > ${path}/heap"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/goroutine > ${path}/goroutine"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/profile > ${path}/profile"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${MONITOR_PORT}"/debug/pprof/metrics > ${path}/metrics"${FILE_NO}".list
    sleep 120
done