#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

if [ -f /root/.kube/config ]; then
  rm /root/.kube/config
fi

ps -a | grep ttyd | xargs kill -9
