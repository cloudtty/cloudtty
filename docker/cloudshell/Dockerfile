FROM node:18.8.0 as builder

# Build frontend code which added upload/download button
WORKDIR /app
COPY html/package.json /app/
COPY html/yarn.lock /app/
RUN yarn config set network-timeout 100000
RUN yarn install
COPY html/ /app/
RUN yarn run build

FROM ghcr.io/dtzar/helm-kubectl:3.9
SHELL [ "/bin/bash", "-c" ]

ARG TTYD_VERSION=1.7.2
ARG ETCDCTL_VERSION=v3.4.20
ARG KREW_VERSION=v0.4.3
ARG KARMACTL_VERSION=v1.4.0

RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> /etc/apk/repositories \
    && apk -U upgrade \
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
    && which ttyd \
    && ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" && echo "Building ARCH of $ARCH.." \
    && curl -fsSLO https://github.com/etcd-io/etcd/releases/download/$ETCDCTL_VERSION/etcd-$ETCDCTL_VERSION-linux-${ARCH}.tar.gz \
    && tar -zxf etcd-$ETCDCTL_VERSION-linux-${ARCH}.tar.gz \
    && chmod +x etcd-$ETCDCTL_VERSION-linux-${ARCH}/etcdctl \
    && mv etcd-$ETCDCTL_VERSION-linux-${ARCH}/etcdctl /usr/local/bin/etcdctl \
    && rm -rf etcd-$ETCDCTL_VERSION-linux-* \
    && curl -fsSLO https://github.com/kubernetes-sigs/krew/releases/download/$KREW_VERSION/krew-linux_${ARCH}.tar.gz \
    && tar -zxf krew-linux_${ARCH}.tar.gz \
    && chmod +x krew-linux_${ARCH} \
    && mv krew-linux_${ARCH} /usr/local/bin/kubectl-krew \
    && kubectl krew version \
    && echo 'PATH="${PATH}:${HOME}/.krew/bin"' >>~/.bashrc \
    && rm -rf krew-linux_* \
    && curl -fsSLO https://github.com/karmada-io/karmada/releases/download/$KARMACTL_VERSION/kubectl-karmada-linux-${ARCH}.tgz \
    && tar -zxf kubectl-karmada-linux-${ARCH}.tgz \
    && chmod +x kubectl-karmada \
    && mv kubectl-karmada ~/.krew/bin/kubectl-karmada \
    && rm -rf kubectl-karmada-linux-*

RUN mkdir /usr/lib/ttyd
COPY --from=builder /app/dist/inline.html /usr/lib/ttyd/index.html
ENTRYPOINT ttyd
WORKDIR /root