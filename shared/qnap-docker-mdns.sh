#!/bin/sh
#
# qnap-docker-mdns control script
# QPKG entrypoint for start, stop, restart, remove
#

QPKG_NAME="qnap-docker-mdns"
DAEMON="qnap-docker-mdnsd"

# qinstall.sh may invoke us before QPKG_ROOT is exported.
if [ -z "${QPKG_ROOT}" ]; then
  QPKG_ROOT="$(dirname "$(readlink -f "$0" 2>/dev/null || readlink "$0" 2>/dev/null || echo "$0")")"
fi

CONFIG_DIR="${QPKG_ROOT}"
DAEMON_BIN="${QPKG_ROOT}/${DAEMON}"
LOCK_FILE="/var/run/${QPKG_NAME}/daemon.lock"

pid_is_running() {
  kill -0 "$1" 2>/dev/null
}

wait_for_exit() {
  PID="$1"
  TIMEOUT="${2:-30}"
  I=0
  while pid_is_running "${PID}"; do
    if [ "${I}" -ge "${TIMEOUT}" ]; then
      return 1
    fi
    sleep 1
    I=$((I + 1))
  done
  return 0
}

case "${1}" in
  start)
    # QDK disabled-package guard
    if [ "${QPKG_DISABLED}" = "TRUE" ]; then
      echo "${QPKG_NAME} is disabled"
      exit 0
    fi

    # Ensure runtime directories exist
    mkdir -p /var/run/${QPKG_NAME}

    # Check if already running
    if [ -f "${LOCK_FILE}" ]; then
      PID=$(cat "${LOCK_FILE}" 2>/dev/null)
      if kill -0 "${PID}" 2>/dev/null; then
        echo "${QPKG_NAME} already running (pid ${PID})"
        exit 0
      fi
    fi

    echo "Starting ${QPKG_NAME}..."
    ${DAEMON_BIN} -config "${CONFIG_DIR}/config.yaml" &
    echo "${QPKG_NAME} started"
    ;;

  stop)
    echo "Stopping ${QPKG_NAME}..."
    if [ -f "${LOCK_FILE}" ]; then
      PID=$(cat "${LOCK_FILE}" 2>/dev/null)
      if pid_is_running "${PID}"; then
        kill "${PID}" 2>/dev/null
        if ! wait_for_exit "${PID}" 30; then
          echo "${QPKG_NAME} did not stop within 30 seconds"
          exit 1
        fi
      fi
    fi
    echo "${QPKG_NAME} stopped"
    ;;

  restart)
    $0 stop
    $0 start
    ;;

  remove)
    echo "Removing ${QPKG_NAME} configuration..."

    # Remove managed JSON entries and regenerate proxy config
    if [ -f /etc/config/reverseproxy/reverseproxy.json ]; then
      # Create backup
      cp /etc/config/reverseproxy/reverseproxy.json \
        "/etc/config/reverseproxy/reverseproxy.json.${QPKG_NAME}.remove.bak"

      # Remove managed entries using jq or sed
      # This is a placeholder - the daemon handles this during shutdown
    fi

    /etc/init.d/reverse_proxy.sh scan_config 2>/dev/null

    rm -rf /var/run/${QPKG_NAME}
    echo "${QPKG_NAME} removed"
    ;;

  *)
    echo "Usage: $0 {start|stop|restart|remove}"
    exit 1
    ;;
esac

exit 0
