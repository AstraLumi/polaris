#!/bin/sh
# Starts Polaris as an ordinary user ("polaris", 10001) instead of root.
#
# The container still begins as root, for one job: stories kept by versions
# before 3.2 are root's files, and so is a brand new volume. They are handed
# to the polaris user here, and then root is given up for good; Polaris
# itself never runs as root.
#
# That job needs three of root's rights (to change a file's owner, and to
# become another user), which docker-compose.yml gives back after dropping
# the rest. Where they are missing (an own compose file with only
# `cap_drop: ALL`), Polaris runs as root as it always did, and says so,
# rather than not start.
#
# Started as some other user already (`user:` in a compose file,
# `docker run --user`): nothing to do, that user runs Polaris.
set -e

RUN_AS=polaris

if [ "$(id -u)" != "0" ]; then
  exec "$@"
fi

# Whether whoever runs the command can make a file in a folder: asked by
# making one (the shell's own "-w" says yes to root even when root's rights
# are gone).
can_write() {
  "$@" touch "$dir/.polaris-write-test" 2>/dev/null && "$@" rm -f "$dir/.polaris-write-test"
}

NEEDS="give the container these rights (docker-compose.yml has them): cap_add: [CHOWN, SETUID, SETGID]"

as_root() {
  echo "polaris: $1" >&2
  shift
  for dir in "$DATA_DIR" "$UPLOADS_DIR"; do
    # Root without its rights can't write another user's files either: the
    # data was handed over on an earlier start and can't be reached now.
    if [ -n "$dir" ] && [ -d "$dir" ] && ! can_write env; then
      echo "polaris: $dir belongs to the $RUN_AS user, and this container can neither become that user nor write to it as root. Nothing was changed. To start, $NEEDS" >&2
      exit 1
    fi
  done
  echo "polaris: running as root, as versions before 3.2 did. To run as an ordinary user, $NEEDS" >&2
  exec "$@"
}

reason=""
if ! su-exec "$RUN_AS" true 2>/dev/null; then
  reason="can't switch to the $RUN_AS user here"
else
  for dir in "$DATA_DIR" "$UPLOADS_DIR"; do
    [ -n "$dir" ] || continue
    mkdir -p "$dir" 2>/dev/null || true
    # Only when something in it isn't the polaris user's yet: on later
    # starts this reads each file's owner and writes nothing.
    if [ -n "$(find "$dir" ! -user "$RUN_AS" -print -quit 2>/dev/null)" ]; then
      if ! chown -R "$RUN_AS:$RUN_AS" "$dir" 2>/dev/null; then
        reason="can't hand $dir to the $RUN_AS user"
        break
      fi
    fi
    if ! can_write su-exec "$RUN_AS"; then
      reason="the $RUN_AS user can't write to $dir"
      break
    fi
  done
fi

if [ -n "$reason" ]; then
  as_root "$reason" "$@"
fi

exec su-exec "$RUN_AS" "$@"
