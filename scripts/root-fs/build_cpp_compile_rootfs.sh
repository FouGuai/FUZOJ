#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROOTFS="${1:-$ROOT_DIR/rootfs/cpp-compile-rootfs}"
SUITE="${2:-noble}"
MIRROR="${3:-http://archive.ubuntu.com/ubuntu}"

if ! command -v debootstrap >/dev/null 2>&1; then
  echo "debootstrap is required. Install it with: sudo apt-get install -y debootstrap" >&2
  exit 1
fi

if [[ -e "$ROOTFS" && ! -d "$ROOTFS" ]]; then
  echo "ROOTFS path exists and is not a directory: $ROOTFS" >&2
  exit 1
fi

sudo mkdir -p "$ROOTFS"

echo "Building rootfs at: $ROOTFS"
sudo debootstrap --variant=minbase "$SUITE" "$ROOTFS" "$MIRROR"

cleanup_mounts() {
  sudo umount -R "$ROOTFS/proc" >/dev/null 2>&1 || true
  sudo umount -R "$ROOTFS/sys" >/dev/null 2>&1 || true
  sudo umount -R "$ROOTFS/dev" >/dev/null 2>&1 || true
}
trap cleanup_mounts EXIT

sudo mount -t proc /proc "$ROOTFS/proc"
sudo mount --rbind /sys "$ROOTFS/sys"
sudo mount --rbind /dev "$ROOTFS/dev"

sudo chroot "$ROOTFS" /bin/bash -lc \
  "apt-get update && apt-get install -y g++ && apt-get clean && rm -rf /var/lib/apt/lists/*"

echo "Rootfs ready: $ROOTFS"
