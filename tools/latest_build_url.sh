#!/usr/bin/env sh

if [ -z "$PROJECT" ]; then
  echo "PROJECT variable not set" >&2
  exit 1
fi

if [ -z "$VERSION" ]; then
  echo "VERSION variable not set" >&2
  exit 1
fi

USER_AGENT="minefleet-gateway/1.0.0 (contact@minefleet.dev)"
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

fetch_builds() {
  curl -s --compressed -H "User-Agent: $USER_AGENT" \
    "https://fill.papermc.io/v3/projects/${PROJECT}/versions/${1}/builds" > "$TMPFILE"
}

fetch_builds "$VERSION"

if jq -e '.ok == false' "$TMPFILE" > /dev/null 2>&1; then
  echo "Error: $(jq -r '.message // "Unknown error"' "$TMPFILE")" >&2
  exit 1
fi

PAPERMC_URL=$(jq -r 'first(.[] | select(.channel == "STABLE") | .downloads."server:default".url) // "null"' "$TMPFILE")

if [ "$PAPERMC_URL" = "null" ]; then
  echo "No stable build for version $VERSION, searching for latest version with stable build..." >&2

  VERSIONS=$(curl -s --compressed -H "User-Agent: $USER_AGENT" \
    "https://fill.papermc.io/v3/projects/${PROJECT}" | \
    jq -r '.versions | to_entries[] | .value[]' | sort -V -r)

  for VERSION in $VERSIONS; do
    fetch_builds "$VERSION"
    STABLE_URL=$(jq -r 'first(.[] | select(.channel == "STABLE") | .downloads."server:default".url) // "null"' "$TMPFILE")
    if [ "$STABLE_URL" != "null" ]; then
      PAPERMC_URL="$STABLE_URL"
      echo "Found stable build for version $VERSION" >&2
      break
    fi
  done
fi

if [ "$PAPERMC_URL" != "null" ]; then
  echo "$PAPERMC_URL"
else
  echo "No stable builds available for any version :(" >&2
  exit 1
fi