FROM node:18.16.0 as builder

# Build frontend code which added upload/download button
WORKDIR /app
COPY html/package.json /app/
COPY html/yarn.lock /app/
RUN yarn config set network-timeout 100000
RUN yarn install
COPY html/ /app/
RUN yarn run build

FROM ghcr.io/dtzar/helm-kubectl:3.19.1
SHELL [ "/bin/bash", "-c" ]

ARG TTYD_VERSION=1.7.8

ENV CFLAGS="-std=gnu89 -Wno-implicit-int"

RUN echo "https://mirrors.aliyun.com/alpine/v3.20/main/" >> /etc/apk/repositories \
    && apk update \
    && apk add wget gcc g++ make bind-tools \
    && apk upgrade libexpat \
    && wget https://ohse.de/uwe/releases/lrzsz-0.12.20.tar.gz \
    && tar -xf lrzsz-0.12.20.tar.gz \
    && cd lrzsz-0.12.20 \
    && ./configure \
    && make \
    && make install \
    && apk add --no-cache ca-certificates \
    && ln -s /usr/local/bin/lrz	/usr/bin/rz \
    && ln -s /usr/local/bin/lsz	/usr/bin/sz \
    && apk add bash-completion \
    && source /usr/share/bash-completion/bash_completion \
    && echo 'source <(kubectl completion bash)' >>~/.bashrc \
    && echo 'alias k=kubectl' >>~/.bashrc \
    && echo 'complete -o default -F __start_kubectl k' >>~/.bashrc

# https://github.com/flpanbin/ttyd/releases/download/1.7.8/ttyd.i686
RUN ttydArch="$(uname -m)" && echo "Building arch of $ttydArch.." \
    && curl -LO https://github.com/cloudtty/ttyd/releases/download/$TTYD_VERSION/ttyd.${ttydArch} \
    && chmod +x ttyd.${ttydArch} \
    && mv ttyd.${ttydArch} /usr/local/bin/ttyd \
    && which ttyd

COPY --from=builder /app/dist/inline.html /usr/lib/ttyd/index.html
COPY ./docker/cloudshell/script/reset.sh  /usr/lib/ttyd/
COPY ./docker/cloudshell/script/startup.sh  /usr/lib/ttyd/

RUN mkdir /root/.kube \
    && chmod +x /usr/lib/ttyd/*.sh \
    && echo 'source /root/.env' >>~/.bashrc \
    && echo '' > .env

USER root
WORKDIR /root
