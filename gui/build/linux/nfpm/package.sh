#!/usr/bin/env bash
# Packages cache22 with nfpm, resolving the version from git tags.
# Usage: package.sh <deb|rpm|archlinux>
# Called by the linux Taskfile; bash is used deliberately so the shell
# expansion below behaves identically everywhere.
set -euo pipefail

cd "$(dirname "$0")/../../.." # gui/

format=${1:?usage: package.sh <deb|rpm|archlinux>}
tag=$(git describe --tags 2>/dev/null || true)
tag=${tag#v}
: "${tag:=0.1.0}"
goarch=$(go env GOARCH)

case "$format" in
deb) out="bin/cache22_${tag}_${goarch}.deb" ;;
rpm) out="bin/cache22-${tag}.${goarch}.rpm" ;;
archlinux) out="bin/cache22-${tag}-${goarch}.pkg.tar.zst" ;;
*) echo "unknown format: $format" >&2; exit 1 ;;
esac

PACKAGE_VERSION="$tag" GOARCH="$goarch" \
	nfpm package -f build/linux/nfpm/nfpm.yaml -p "$format" -t "$out"
