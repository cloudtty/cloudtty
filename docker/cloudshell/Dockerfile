FROM node:18.16.0 as builder

# Build frontend code which added upload/download button
WORKDIR /app
COPY html/package.json /app/
COPY html/yarn.lock /app/
RUN yarn config set network-timeout 100000
RUN yarn install
COPY html/ /app/
RUN yarn run build

FROM ghcr.io/dtzar/helm-kubectl:3.13.2
SHELL [ "/bin/bash", "-c" ]

ARG TTYD_VERSION=1.7.4

RUN echo "https://mirrors.aliyun.com/alpine/edge/testing/" >> /etc/apk/repositories \
    && apk update \
    && apk add --no-cache ca-certificates lrzsz \
    && ln -s /usr/bin/lrz	/usr/bin/rz \
    && ln -s /usr/bin/lsz	/usr/bin/sz \
    && apk add bash-completion \
    && source /usr/share/bash-completion/bash_completion \
    && echo 'source <(kubectl completion bash)' >>~/.bashrc \
    && echo 'alias k=kubectl' >>~/.bashrc \
    && echo 'complete -o default -F __start_kubectl k' >>~/.bashrc

RUN ttydArch="$(uname -m)" && echo "Building arch of $ttydArch.." \
    && curl -LO https://github.com/tsl0922/ttyd/releases/download/$TTYD_VERSION/ttyd.${ttydArch} \
    && chmod +x ttyd.${ttydArch} \
    && mv ttyd.${ttydArch} /usr/local/bin/ttyd \
    && which ttyd

RUN mkdir /root/.kube
RUN mkdir /usr/lib/ttyd
COPY --from=builder /app/dist/inline.html /usr/lib/ttyd/index.html
COPY ./docker/cloudshell/script/reset.sh  /usr/lib/ttyd/
COPY ./docker/cloudshell/script/startup.sh  /usr/lib/ttyd/
RUN chmod +x /usr/lib/ttyd/*.sh

WORKDIR /root
