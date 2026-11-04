#!/usr/bin/env bash
# Package committed configuration with no Go source and no possible host access.
set -euo pipefail
root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
base=$(mktemp -d)
trap 'rm -rf -- "$base"' EXIT
repo="$base/config"
mkdir "$repo"
git -C "$root" archive --format=zip HEAD -- internal/deploy/targets.json servers/current > "$base/config.zip"
unzip -q "$base/config.zip" -d "$repo"
[[ -z $(find "$repo" -name '*.go' -print -quit) ]]
git -C "$repo" init --initial-branch=main --quiet
git -C "$repo" config user.name 'Configuration integration test'
git -C "$repo" config user.email config-test@example.invalid
git -C "$repo" config core.autocrlf false
printf '.tools/\n' > "$repo/.gitignore"
git -C "$repo" add .
git -C "$repo" commit --quiet -m 'Configuration fixture'
git clone --bare --quiet "$repo" "$base/origin.git"
git -C "$repo" remote add origin "$base/origin.git"
mkdir -p "$repo/.tools"
cp -R "$root/.tools/dzcli" "$repo/.tools/"
for alias in nasdara nasdara-staging deer-isle; do
    missing="$repo/intentionally-missing-client.yaml"
    # Client YAML is loaded after packaging/validation and before any SSH access.
    code=0
    "$root/.tools/bin/dzdeploy" deploy "$alias" --repo "$repo" --dry-run --config "$missing" > "$base/result" 2>&1 || code=$?
    if [[ $code != 1 ]] || ! grep -Fq 'load local endpoint config' "$base/result" || ! grep -Fq "$missing" "$base/result"; then
        cat "$base/result" >&2; exit 1
    fi
    [[ -n $(find "$repo/.tools/dzdeploy/build" -name serverDZ.cfg -print -quit) ]]
    echo "$alias: committed packaging and validation passed without Go or host access"
done
