FROM nginxinc/nginx-unprivileged:1.27

USER root

RUN mkdir -p /usr/lib/ttyd /root/.kube

COPY test/e2e/fixtures/cloudshell/startup.sh /usr/lib/ttyd/startup.sh
COPY test/e2e/fixtures/cloudshell/reset.sh /usr/lib/ttyd/reset.sh

RUN chmod +x /usr/lib/ttyd/startup.sh /usr/lib/ttyd/reset.sh
