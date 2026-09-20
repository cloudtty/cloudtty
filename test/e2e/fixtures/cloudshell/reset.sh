#!/bin/bash

set -euo pipefail

nginx -s stop -c /tmp/cloudtty-nginx.conf 2>/dev/null || true
