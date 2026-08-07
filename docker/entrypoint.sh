#!/bin/sh
set -eu

# Shared session bus so `docker exec ... proton-drive auth login` and
# backup uploads see the same libsecret collection for the container lifetime.
DBUS_PATH="${HOME:-/data}/.dbus"
DBUS_SOCK="${DBUS_PATH}/bus"
DBUS_ENV="${DBUS_PATH}/session.env"

mkdir -p "${DBUS_PATH}"

if [ ! -S "${DBUS_SOCK}" ]; then
  dbus-daemon --session --fork --address="unix:path=${DBUS_SOCK}" --nopidfile
fi

export DBUS_SESSION_BUS_ADDRESS="unix:path=${DBUS_SOCK}"
printf 'export DBUS_SESSION_BUS_ADDRESS=%s\n' "${DBUS_SESSION_BUS_ADDRESS}" > "${DBUS_ENV}"

exec /usr/local/bin/rclonarr "$@"
