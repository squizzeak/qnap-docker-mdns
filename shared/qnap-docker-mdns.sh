#!/bin/sh
#
# qnap-docker-mdns control script
# QPKG entrypoint for start, stop, restart, remove
#

QPKG_NAME="qnap-docker-mdns"
DAEMON="qnap-docker-mdnsd"
CONFIG_DIR="${QPKG_ROOT}"
DAEMON_BIN="${QPKG_ROOT}/${DAEMON}"
LOCK_FILE="/var/run/${QPKG_NAME}/daemon.lock"

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
      kill "${PID}" 2>/dev/null
      sleep 1
    fi
    echo "${QPKG_NAME} stopped"
    ;;

  restart)
    $0 stop
    sleep 1
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
