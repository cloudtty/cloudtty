#!/bin/bash

set -euo pipefail

cat >/tmp/cloudtty-nginx.conf <<'EOF'
pid /tmp/cloudtty-nginx.pid;
error_log /dev/stderr info;

events {}

http {
  access_log /dev/stdout;
  server {
    listen 7681;
    location / {
      default_type text/plain;
      return 200 "cloudtty gateway api e2e\n";
    }
  }
}
EOF

nginx -c /tmp/cloudtty-nginx.conf -g 'daemon off;' >/tmp/cloudtty-nginx.log 2>&1 &
