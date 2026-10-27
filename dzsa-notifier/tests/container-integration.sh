#!/bin/sh
set -eu
image=$1
fixture=$2
work=$(mktemp -d)
"$fixture" --directory "$work" >"$work/fixture.log" 2>&1 &
fixture_pid=$!
trap 'kill "$fixture_pid" 2>/dev/null || true' EXIT
chmod 0755 "$work"
for i in $(seq 1 50); do [ -f "$work/ca.pem" ] && break; sleep .1; done
test "$(docker image inspect "$image" --format '{{.Config.User}}')" = 65532:65532
test "$(docker run --rm --entrypoint /bin/sh "$image" -c true 2>/dev/null; echo $?)" != 0
set +e
docker run --rm --user=0:0 -e DZSA_NOTIFIER_ENDPOINTS=203.0.113.10:27016 "$image" --once
root_status=$?
set -e
test "$root_status" -eq 77
# Native and emulated arm64 images both perform real, verified HTTPS to the fixture.
docker run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges --network=host \
    -v "$work/ca.pem:/test-ca.pem:ro" -e SSL_CERT_FILE=/test-ca.pem \
    -e HTTPS_PROXY=http://127.0.0.1:18443 -e DZSA_NOTIFIER_ENDPOINTS=203.0.113.10:27016 \
    "$image" --once
docker save "$image" -o "$work/image.tar"
podman load -i "$work/image.tar"
test "$(podman info --format '{{.Host.Security.Rootless}}')" = true
podman run --rm --read-only --cap-drop=ALL --security-opt=no-new-privileges --network=host \
    -v "$work/ca.pem:/test-ca.pem:ro" -e SSL_CERT_FILE=/test-ca.pem \
    -e HTTPS_PROXY=http://127.0.0.1:18443 -e DZSA_NOTIFIER_ENDPOINTS=203.0.113.10:27019 \
    "$image" --once
echo "$image nonroot/distroless/rootless HTTPS checks passed"
