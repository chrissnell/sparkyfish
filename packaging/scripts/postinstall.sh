#!/bin/sh
set -e

# Create sparkyfish user/group via systemd-sysusers if available,
# otherwise fall back to useradd/groupadd
if command -v systemd-sysusers >/dev/null 2>&1; then
    systemd-sysusers sparkyfish-server.conf
elif ! getent passwd sparkyfish >/dev/null 2>&1; then
    groupadd --system sparkyfish 2>/dev/null || true
    useradd --system --gid sparkyfish --shell /usr/sbin/nologin \
        --home-dir / --no-create-home sparkyfish
fi

if [ -d /run/systemd/system ]; then
    systemctl daemon-reload
fi
