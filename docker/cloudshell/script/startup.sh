#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KUBECONFIG=${1:-}
ONCE=${2:-}
URLARG=${3:-}
COMMAND=${4:-"bash"}
POD_NAME=${5:-}
POD_NAMESPACE=${6:-"default"}
CONTAINER=${7:-}
PS1=${8:-}
SERVER_BUFFER_SIZE=${9:-}

if [ -d /root -a "`ls /root`" != "" ]; then         
  rm -rf /root/*                                    
fi

## Generate config to the path `/root/.kube/config`.
if [[ -n "${KUBECONFIG}"  ]]; then
  echo "${KUBECONFIG}" > /root/.kube/config
fi

echo "export POD_NAMESPACE='${POD_NAMESPACE}'" > /root/.env

if [[ -n "${POD_NAME}" ]]; then
  echo "export POD_NAME='${POD_NAME}'" >> /root/.env
fi

if [[ -n "${CONTAINER}" ]]; then
  echo "export CONTAINER='${CONTAINER}'" >> /root/.env
fi

if [[ -n "${PS1}" ]]; then
  echo "export PS1='${PS1}'" >> /root/.env
fi

source /root/.bashrc

once=""
index=""
urlarg=""
server_buffer_size=""

if [[ "${ONCE}" == "true" ]];then
  once=" --once "
fi

if [[ -f /usr/lib/ttyd/index.html ]]; then
  index=" --index /usr/lib/ttyd/index.html "
fi

if [[ "${URLARG}" == "true" ]];then
  urlarg=" -a "
fi

if [[ -n "${SERVER_BUFFER_SIZE}" ]]; then
  server_buffer_size=" --serv_buffer_size ${SERVER_BUFFER_SIZE} "
fi

nohup ttyd -W ${index} ${once} ${urlarg} ${server_buffer_size} sh -c "${COMMAND}" > /usr/lib/ttyd/nohup.log 2>&1 &
echo "Start ttyd successully."
