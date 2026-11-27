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
ARG TARGETARCH
ARG HELM_VERSION=3.22.1
ARG KUBECTL_VERSION=1.34.5
ARG YQ_VERSION=4.53.6

ENV CFLAGS="-std=gnu89 -Wno-implicit-int"

RUN apk update \
    && apk add wget gcc g++ make bind-tools \
    && apk upgrade --no-cache libexpat bind-libs bind-tools libcrypto3 libssl3 \
        openssh openssh-client-common openssh-client-default openssh-keygen \
        openssh-server openssh-server-common openssh-sftp-server \
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

RUN set -eu; \
    tmp_dir=$(mktemp -d); \
    trap 'rm -rf "$tmp_dir"' EXIT; \
    helm_archive="helm-v${HELM_VERSION}-linux-${TARGETARCH}.tar.gz"; \
    curl --fail --show-error --location --retry 3 --connect-timeout 15 --max-time 180 \
        --output "$tmp_dir/$helm_archive" "https://get.helm.sh/$helm_archive"; \
    curl --fail --show-error --location --retry 3 --connect-timeout 15 --max-time 180 \
        --output "$tmp_dir/$helm_archive.sha256sum" "https://get.helm.sh/$helm_archive.sha256sum"; \
    (cd "$tmp_dir" && sha256sum -c "$helm_archive.sha256sum"); \
    tar -xzf "$tmp_dir/$helm_archive" -C "$tmp_dir" "linux-${TARGETARCH}/helm"; \
    install -m 0755 "$tmp_dir/linux-${TARGETARCH}/helm" /usr/local/bin/helm; \
    curl --fail --show-error --location --retry 3 --connect-timeout 15 --max-time 180 \
        --output "$tmp_dir/kubectl" "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl"; \
    curl --fail --show-error --location --retry 3 --connect-timeout 15 --max-time 180 \
        --output "$tmp_dir/kubectl.sha256" "https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/${TARGETARCH}/kubectl.sha256"; \
    (cd "$tmp_dir" && echo "$(cat kubectl.sha256)  kubectl" | sha256sum -c -); \
    install -m 0755 "$tmp_dir/kubectl" /usr/local/bin/kubectl; \
    curl --fail --show-error --location --retry 3 --connect-timeout 15 --max-time 180 \
        --output "$tmp_dir/yq.tar.gz" "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_${TARGETARCH}.tar.gz"; \
    tar -xzf "$tmp_dir/yq.tar.gz" -C "$tmp_dir"; \
    install -m 0755 "$tmp_dir/yq_linux_${TARGETARCH}" /usr/local/bin/yq

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
