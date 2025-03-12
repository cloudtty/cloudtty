#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -f /root/.kube/config ]]; then
  rm  /root/.kube/config -f
fi

if [[ -f /root/.env ]]; then
  rm  /root/.env -f
fi

if [[ -f /root/.ash_history ]]; then
  rm  /root/.ash_history -f
fi

if [[ -f /root/.bash_history ]]; then
  rm  /root/.bash_history -f
fi

ps aux | grep 'ttyd -W' | grep -v 'grep' | awk '{print $1}' | xargs kill -9 > /dev/null 2>&1
