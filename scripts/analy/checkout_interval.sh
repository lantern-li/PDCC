#!/usr/bin/env bash

function checkoutInterval() {
  # 检索关键日志
  grepped_log="commit_block_log"
  rm -rf $grepped_log
  grep -a "commit block \[" $LOG > $grepped_log

  # 检索interval大于阈值的日志
  while read line
     do
       # 检索出interval
       str=${line}
       interval="interval:"
       regex=".*"${interval}"([0-9]+).*"
       val=$(echo "$str"|gawk '{print gensub("'"$regex"'","\\1","1")}')
       # 如果大于阈值则打印
       if [[ $val -ge $THRESHOLD ]] ; then
         echo "$str"
       fi
     done < $grepped_log
  # 清除临时日志
  rm -rf $grepped_log
}


# 第一个参数是日志路径
LOG=$1
# 第二个参数是interval阈值，单位ms
THRESHOLD=$2

# ./checkout_interval.sh system.log 30000

checkoutInterval