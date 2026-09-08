#!/bin/sh
set -e

# Velocity treats velocity.toml as mutable state: on startup it migrates the
# config between versions and writes the result back in place. ConfigMap and
# Secret volumes are always mounted read-only by the kubelet, so a config
# mounted straight onto /velocity/velocity.toml makes that write fail with
# EROFS and the proxy exits before it finishes starting.
#
# Anything mounted at /config is therefore staged into the working directory,
# which is writable, so Velocity can rewrite it. The copy is per-container and
# lost on restart, which is what we want: the mounted source stays the only
# thing that persists.
if [ -d /config ]; then
    for file in /config/*; do
        # An empty directory leaves the glob unexpanded; -e filters that out
        # along with the ..data symlinks a ConfigMap projection adds.
        if [ -e "$file" ]; then
            target="/velocity/$(basename "$file")"
            if ! cp -L "$file" "$target"; then
                echo "minefleet: cannot stage $(basename "$file") into /velocity." >&2
                echo "minefleet: $target is most likely still mounted directly from a" >&2
                echo "minefleet: ConfigMap or Secret, which is always read-only. Mount the" >&2
                echo "minefleet: volume at /config instead and remove the direct mount." >&2
                exit 1
            fi
        fi
    done
fi

exec "$@"
