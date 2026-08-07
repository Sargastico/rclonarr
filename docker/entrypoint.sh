#!/bin/sh
set -eu

# Proton Drive CLI uses libsecret and expects the "login" collection at
# /org/freedesktop/secrets/collection/login. Provide a fresh D-Bus session +
# unlocked gnome-keyring each start. Keyring files persist under HOME=/data.
HOME="${HOME:-/data}"
export HOME

DBUS_PATH="${HOME}/.dbus"
DBUS_SOCK="${DBUS_PATH}/bus"
DBUS_ENV="${DBUS_PATH}/session.env"

mkdir -p "${DBUS_PATH}" \
	"${HOME}/.local/share/keyrings" \
	"${HOME}/.cache"

# Stale sockets from previous container PIDs break session reuse.
rm -f "${DBUS_SOCK}"
dbus-daemon --session --fork --address="unix:path=${DBUS_SOCK}" --nopidfile
export DBUS_SESSION_BUS_ADDRESS="unix:path=${DBUS_SOCK}"

# Empty-password login keyring. Order matters for creating collection/login.
printf '\0\0' | gnome-keyring-daemon --login --components=secrets >/dev/null
eval "$(gnome-keyring-daemon --start --components=secrets)"
printf '\n' | gnome-keyring-daemon --replace --unlock >/dev/null || true
if [ -z "${GNOME_KEYRING_CONTROL:-}" ]; then
	eval "$(gnome-keyring-daemon --start --components=secrets)"
fi

{
	printf 'export HOME=%s\n' "${HOME}"
	printf 'export DBUS_SESSION_BUS_ADDRESS=%s\n' "${DBUS_SESSION_BUS_ADDRESS}"
	if [ -n "${GNOME_KEYRING_CONTROL:-}" ]; then
		printf 'export GNOME_KEYRING_CONTROL=%s\n' "${GNOME_KEYRING_CONTROL}"
	fi
} > "${DBUS_ENV}"

exec /usr/local/bin/rclonarr "$@"
