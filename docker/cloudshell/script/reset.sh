#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -f /root/.kube/config ]]; then
  rm  /root/.kube/config -f
fi

ps aux | grep 'ttyd -W' | awk '{print $1}' | xargs kill -9 > /dev/null 2>&1
