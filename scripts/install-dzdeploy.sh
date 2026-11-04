#!/usr/bin/env bash
set -euo pipefail

version=0.1.0
checksums_sha256=ecf38581f7839f7dfa5bfcd28ecdf50b6cc65adbbaf55e70a53f600516bfdbcd
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
[[ $(uname -s) == Linux ]] || { echo 'This installer requires Linux.' >&2; exit 1; }
case $(uname -m) in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo 'Only amd64 and arm64 are supported.' >&2; exit 1 ;;
esac
for tool in curl sha256sum unzip; do command -v "$tool" >/dev/null; done
mkdir -p "$root/.tools"
stage=$(mktemp -d "$root/.tools/dzdeploy-install-XXXXXXXX")
trap 'rm -rf -- "$stage"' EXIT
base="https://github.com/jsirianni/dzdeploy/releases/download/v$version"
archive="dzdeploy_${version}_linux_${arch}.zip"
curl --fail --location --silent --show-error --max-time 120 "$base/checksums.txt" -o "$stage/checksums.txt"
printf '%s  %s\n' "$checksums_sha256" "$stage/checksums.txt" | sha256sum --check --status
curl --fail --location --silent --show-error --max-time 120 "$base/$archive" -o "$stage/$archive"
expected=$(awk -v name="$archive" '$2 == name { print $1 }' "$stage/checksums.txt")
[[ $expected =~ ^[0-9a-f]{64}$ ]] || { echo 'Missing archive checksum.' >&2; exit 1; }
printf '%s  %s\n' "$expected" "$stage/$archive" | sha256sum --check --status
# Extract only known files from the verified archive, preserving the companion.
files=(dzdeploy agents/windows-amd64/dzdeploy.exe bootstrap-windows.ps1 deploy.example.yaml deployment.md operator-workflows.md)
for file in "${files[@]}"; do
    mkdir -p "$stage/package/$(dirname -- "$file")"
    unzip -p "$stage/$archive" "$file" > "$stage/package/$file"
    [[ -s "$stage/package/$file" ]]
done
chmod 755 "$stage/package/dzdeploy" "$stage/package/agents/windows-amd64/dzdeploy.exe"
for file in "${files[@]}"; do
    mkdir -p "$root/.tools/bin/$(dirname -- "$file")"
    mv -f -- "$stage/package/$file" "$root/.tools/bin/$file"
done
echo "Installed dzdeploy v$version: $root/.tools/bin/dzdeploy"
