#!/usr/bin/env bash
# Packages cache22 with nfpm, resolving the version from git tags.
# Usage: package.sh <deb|rpm|archlinux>
# Called by the linux Taskfile; bash is used deliberately so the shell
# expansion below behaves identically everywhere.
set -euo pipefail

cd "$(dirname "$0")/../../.." # gui/

format=${1:?usage: package.sh <deb|rpm|archlinux>}
case "$format" in
deb) out=bin/cache22.deb ;;
rpm) out=bin/cache22.rpm ;;
archlinux) out=bin/cache22.pkg.tar.zst ;;
*) echo "unknown format: $format" >&2; exit 1 ;;
esac

tag=$(git describe --tags 2>/dev/null || true)
tag=${tag#v}
: "${tag:=0.1.0}"

PACKAGE_VERSION="$tag" GOARCH="$(go env GOARCH)" \
	nfpm package -f build/linux/nfpm/nfpm.yaml -p "$format" -t "$out"
