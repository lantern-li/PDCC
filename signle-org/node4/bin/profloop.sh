#!/bin/bash

IP=$1
MONITOR_PORT=$2
PPROF_PORT=$2
OUT_PATH=$2

if [ -z "$ip" ]; then
    echo "please specify ip:port: [usage] heap.sh ip:port"
    exit
fi

if [ -z "${OUT_PATH}" ]; then
    OUT_PATH=/tmp/prof
fi
mkdir -p OUT_PATH
while :
do
    FILE_NO=$(date "+%Y-%m-%d_%H:%M:%S")
    echo "$FILE_NO"
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/heap > ${OUT_PATH}/heap"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/goroutine > ${OUT_PATH}/goroutine"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${PPROF_PORT}"/debug/pprof/profile > ${OUT_PATH}/profile"${FILE_NO}".prof
    curl -sK -v  http://"${IP}:${MONITOR_PORT}"/debug/pprof/metrics > ${OUT_PATH}/metrics"${FILE_NO}".list
    sleep 120
done