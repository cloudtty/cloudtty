# Keep the Helm major version stable while replacing the vulnerable bundled binary.
FROM alpine:3.22 AS helm
ARG TARGETARCH
ARG HELM_VERSION=3.21.0
RUN apk add --no-cache curl \
    && arch="${TARGETARCH:-$(apk --print-arch)}" \
    && case "$arch" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; esac \
    && archive="helm-v${HELM_VERSION}-linux-${arch}.tar.gz" \
    && curl -fsSLo "$archive" "https://get.helm.sh/$archive" \
    && curl -fsSLo "$archive.sha256sum" "https://get.helm.sh/$archive.sha256sum" \
    && sha256sum -c "$archive.sha256sum" \
    && tar -xzf "$archive" \
    && install -m 0755 "linux-${arch}/helm" /usr/local/bin/helm

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
COPY --from=helm /usr/local/bin/helm /usr/local/bin/helm
SHELL [ "/bin/bash", "-c" ]

ARG TTYD_VERSION=1.7.8

ENV CFLAGS="-std=gnu89 -Wno-implicit-int"

# Upgrade against the rootfs branch; mixing in v3.20 leaves newer packages unpatched.
RUN ALPINE_BRANCH="v$(cut -d. -f1,2 /etc/alpine-release)" \
    && printf '%s\n' \
        "https://dl-cdn.alpinelinux.org/alpine/${ALPINE_BRANCH}/main" \
        "https://dl-cdn.alpinelinux.org/alpine/${ALPINE_BRANCH}/community" \
        > /etc/apk/repositories \
    && apk update \
    && apk add wget gcc g++ make bind-tools \
    && apk upgrade --no-cache \
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
