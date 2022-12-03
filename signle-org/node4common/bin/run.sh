#!/usr/bin/env bash

start() {
	export LD_LIBRARY_PATH=$(dirname $PWD)/lib:$LD_LIBRARY_PATH
  export PATH=$(dirname $PWD)/lib:$PATH
  export WASMER_BACKTRACE=1
  pid=`ps -ef | grep chainmaker | grep "\-c ../config/wx-org.chainmaker.org/chainmaker.yml" | grep -v grep |  awk  '{print $2}'`
  if [ -z ${pid} ];then
      nohup ./chainmaker start -c ../config/wx-org.chainmaker.org/chainmaker.yml > panic.log &
      echo "chainmaker is starting, pls check log..."
  else
      echo "chainmaker is already started"
  fi
}

stop() {
  pid=`ps -ef | grep chainmaker | grep "\-c ../config/wx-org.chainmaker.org/chainmaker.yml" | grep -v grep |  awk  '{print $2}'`
  if [ ! -z ${pid} ];then
      kill $pid
  fi
  echo "chainmaker is stopped"
}

case "$1" in
    start)
      start
    	;;
    stop)
      stop
    	;;
    restart)
    	echo "chainmaker restart"
    	stop
    	start
    	;;
    *)
        echo "you can use: $0 [start|stop|restart]"
	exit 1 
esac

exit 0
