FROM --platform=${BUILDPLATFORM} node:18.8.0 as builder

# Build frontend code which added upload/download button
WORKDIR /app
COPY html/package.json /app/
COPY html/yarn.lock /app/
RUN yarn install
COPY html/ /app/
RUN yarn run build

FROM ghcr.io/dtzar/helm-kubectl:3.9
SHELL [ "/bin/bash", "-c" ]

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
    && curl -LO https://github.com/tsl0922/ttyd/releases/download/1.6.3/ttyd.${ttydArch} \
    && chmod +x ttyd.${ttydArch} \
    && mv ttyd.${ttydArch} /usr/local/bin/ttyd \
    && which ttyd \
    && mkdir kubeconf \
    && ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" && echo "Building ARCH of $ARCH.." \
    && curl -fsSLO https://github.com/karmada-io/karmada/releases/download/v1.2.0/kubectl-karmada-linux-${ARCH}.tgz \
    && tar -zxf kubectl-karmada-linux-${ARCH}.tgz \
    && chmod +x kubectl-karmada \
    && mv kubectl-karmada /usr/local/bin/kubectl-karmada \
    && which kubectl-karmada \
    && rm -rf kubectl-karmada-linux-* \
    && curl -fsSLO https://github.com/etcd-io/etcd/releases/download/v3.4.20/etcd-v3.4.20-linux-${ARCH}.tar.gz \
    && tar -zxf etcd-v3.4.20-linux-${ARCH}.tar.gz \
    && chmod +x etcd-v3.4.20-linux-${ARCH}/etcdctl \
    && mv etcd-v3.4.20-linux-${ARCH}/etcdctl /usr/local/bin/etcdctl \
    && which kubectl-karmada \
    && rm -rf etcd-v3.4.20-linux-*

COPY --from=builder /app/dist/inline.html index.html
ENTRYPOINT ttyd