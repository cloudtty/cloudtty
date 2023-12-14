#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

TTL=${1:-}
ONCE=${2:-}
URLARG=${3:-}
COMMAND=${4:-"bash"}

once=""
index=""
urlarg=""

if [ "${ONCE}" == "true" ];then
  once=" --once "
fi

if [ -f /usr/lib/ttyd/index.html ]; then
  index=" --index /usr/lib/ttyd/index.html "
fi

if [ "${URLARG}" == "true" ];then
  urlarg=" -a "
fi

nohup ttyd -W ${index} ${once} ${urlarg} sh -c "${COMMAND}" >  /usr/lib/ttyd/nohup.log 2>&1 &

echo "Start ttyd success"
