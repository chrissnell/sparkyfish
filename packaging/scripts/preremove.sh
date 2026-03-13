#!/bin/sh
set -e

if [ -d /run/systemd/system ]; then
    systemctl stop sparkyfish-server 2>/dev/null || true
    systemctl disable sparkyfish-server 2>/dev/null || true
fi
