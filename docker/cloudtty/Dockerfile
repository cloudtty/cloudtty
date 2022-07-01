FROM node:18.5.0 as builder
# Build frontend code which added upload/download button
WORKDIR /app
COPY html/package.json /app/
COPY html/yarn.lock /app/
RUN yarn install
COPY html/ /app/
RUN yarn run build

FROM ghcr.io/dtzar/helm-kubectl:3.9

RUN echo "https://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> /etc/apk/repositories \
    && apk -U upgrade \
    && apk add --no-cache ca-certificates lrzsz \
    && ln -s /usr/bin/lrz	/usr/bin/rz \
    && ln -s /usr/bin/lsz	/usr/bin/sz \
    && curl -LO https://github.com/tsl0922/ttyd/releases/download/1.6.3/ttyd.x86_64 \
    && chmod +x ttyd.x86_64 \
    && mv ttyd.x86_64 /usr/local/bin/ttyd \
    && which ttyd \
    && mkdir kubeconf

COPY --from=builder /app/dist/inline.html index.html
ENTRYPOINT ttyd
