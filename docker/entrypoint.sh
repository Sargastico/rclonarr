#!/bin/sh
set -eu

# Proton Drive CLI uses libsecret (org.freedesktop.secrets). Provide a shared
# D-Bus session + gnome-keyring for the container lifetime, persisted under HOME.
HOME="${HOME:-/data}"
export HOME

DBUS_PATH="${HOME}/.dbus"
DBUS_SOCK="${DBUS_PATH}/bus"
DBUS_ENV="${DBUS_PATH}/session.env"
KEYRING_DIR="${HOME}/.local/share/keyrings"
CACHE_DIR="${HOME}/.cache"

mkdir -p "${DBUS_PATH}" "${KEYRING_DIR}" "${CACHE_DIR}"

if [ ! -S "${DBUS_SOCK}" ]; then
  dbus-daemon --session --fork --address="unix:path=${DBUS_SOCK}" --nopidfile
fi
export DBUS_SESSION_BUS_ADDRESS="unix:path=${DBUS_SOCK}"

# Start Secret Service if it is not already on this bus.
if ! dbus-send --session \
    --dest=org.freedesktop.DBus \
    --type=method_call \
    --print-reply \
    /org/freedesktop/DBus org.freedesktop.DBus.ListNames 2>/dev/null \
    | grep -q 'org.freedesktop.secrets'; then
  # Prints GNOME_KEYRING_CONTROL=... for child processes / proton-drive.
  eval "$(gnome-keyring-daemon --start --components=secrets)"
  # Unlock with an empty password (container-local keyring under HOME=/data).
  printf '' | gnome-keyring-daemon --unlock 2>/dev/null || true
fi

{
  printf 'export HOME=%s\n' "${HOME}"
  printf 'export DBUS_SESSION_BUS_ADDRESS=%s\n' "${DBUS_SESSION_BUS_ADDRESS}"
  if [ -n "${GNOME_KEYRING_CONTROL:-}" ]; then
    printf 'export GNOME_KEYRING_CONTROL=%s\n' "${GNOME_KEYRING_CONTROL}"
  fi
} > "${DBUS_ENV}"

exec /usr/local/bin/rclonarr "$@"
