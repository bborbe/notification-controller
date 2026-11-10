#!/usr/bin/env bash
# Provisions the cluster-side prerequisites for a remote build: the `build`
# namespace and the docker.io push secret the Job mounts. Idempotent — running
# it twice exits 0 and leaves exactly one namespace and one secret.
#
# Called by hand before the first `make build BUILD_TARGET=<cluster>` against a
# cluster. It deliberately creates no Job: jobs are per-build, rendered by
# hack/build-cluster.sh, which refuses to run when one already exists.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE="${TEMPLATE:-${SCRIPT_DIR}/dockerconfig.json.tpl}"

BUILD_TARGET="${1:-}"
NAMESPACE="${NAMESPACE:-build}"
BUILD_PUSH_SECRET="${BUILD_PUSH_SECRET:-buildkit-push}"

die() {
	echo "build-cluster-bootstrap: $*" >&2
	exit 1
}

[ -n "${BUILD_TARGET}" ] || die "usage: $(basename "${BASH_SOURCE[0]}") <sun|nukedev|nukeprod|kubectl-context>"
[ -f "${TEMPLATE}" ] || die "template not found: ${TEMPLATE}"

# The same -teamvault-config value `make apply` takes (Makefile.k8s). Defaults to
# the config the repo's own .envrc uses, so an operator who has teamvault-cli set
# up does not have to export anything.
TEAMVAULT="${TEAMVAULT:-${HOME}/.config/teamvault-cli/config.json}"
[ -f "${TEAMVAULT}" ] || die "teamvault config not found: ${TEAMVAULT} (set TEAMVAULT to override)"
command -v teamvault-cli >/dev/null 2>&1 ||
	die "teamvault-cli not found on PATH; it resolves the lookup key in ${TEMPLATE}"

# Shared with hack/build-cluster.sh so the alias mapping cannot drift between
# provisioning and building.
source "${SCRIPT_DIR}/cluster-alias.sh"

# The rendered credential lives in a 600 temp file for the length of one
# `create secret --dry-run` and is removed on every exit path. It is never
# echoed, never logged, and never written into the repo.
RENDERED="$(mktemp)"
chmod 600 "${RENDERED}"
trap 'rm -f "${RENDERED}"' EXIT INT TERM

echo "build-cluster-bootstrap: cluster ${BUILD_TARGET} (kubectl context ${KUBE_CONTEXT})"

kc create namespace "${NAMESPACE}" --dry-run=client -o yaml | kc apply -f -
echo "build-cluster-bootstrap: namespace ${NAMESPACE} present"

# The lookup key comes from the template, so the template stays the single
# declaration of which TeamVault item and which registry this secret is for.
#
# awk, not a `sed` pipeline truncated by `head`: `head` exits after the first
# match, so a template carrying a second `teamvaultUser` occurrence sends
# SIGPIPE to `sed`, which fails the pipeline under `pipefail` before the `die`
# below can report the real cause. `hack/build-cluster.sh` replaced the same
# shape for the same reason.
TEAMVAULT_KEY="$(awk '
	{
		if (match($0, /"[A-Za-z0-9]{6,12}" \| teamvaultUser/)) {
			key = substr($0, RSTART + 1, RLENGTH - 1)
			sub(/".*/, "", key)
			print key
			exit
		}
	}
' "${TEMPLATE}")"
[ -n "${TEAMVAULT_KEY}" ] || die "no teamvaultUser lookup key found in ${TEMPLATE}"

# teamvault-cli, not teamvault-config-parser: the parser takes its password from
# the config file, which holds only url and user, so it authenticates unprivileged
# and gets 403 on this item. The CLI reads the login from the macOS Keychain.
TV_USER="$(teamvault-cli username --teamvault-config "${TEAMVAULT}" --teamvault-key="${TEAMVAULT_KEY}")"
TV_PASS="$(teamvault-cli password --teamvault-config "${TEAMVAULT}" --teamvault-key="${TEAMVAULT_KEY}")"
[ -n "${TV_USER}" ] && [ -n "${TV_PASS}" ] || die "teamvault returned an empty credential for key ${TEAMVAULT_KEY}"

# docker.io's canonical auths key, per the template. Render through jq rather
# than printf: a credential containing a quote, a backslash or a newline makes
# raw interpolation produce malformed JSON — a backslash can escape the closing
# quote and let a crafted value inject fields into the dockerconfigjson.
#
# The password reaches jq as a file, never as `--arg`: an argv argument is
# readable by every local process through `ps` or /proc/<pid>/cmdline for the
# life of the process, which on a shared build host is an exposure the 600
# tempfile below does not cover. The username is not secret and stays an `--arg`.
command -v jq >/dev/null 2>&1 || die "jq is required to render ${TEMPLATE} safely"
jq -n --arg user "${TV_USER}" --rawfile pass <(printf '%s' "${TV_PASS}") \
	'{auths:{"https://index.docker.io/v1/":{username:$user,password:$pass}}}' \
	>"${RENDERED}"
unset TV_USER TV_PASS
[ -s "${RENDERED}" ] || die "failed to render ${TEMPLATE}"

kc create secret generic "${BUILD_PUSH_SECRET}" \
	-n "${NAMESPACE}" \
	--type=kubernetes.io/dockerconfigjson \
	--from-file=.dockerconfigjson="${RENDERED}" \
	--dry-run=client -o yaml | kc apply -f -
echo "build-cluster-bootstrap: secret ${NAMESPACE}/${BUILD_PUSH_SECRET} present"

echo "build-cluster-bootstrap: ready — make build BUILD_TARGET=${BUILD_TARGET}"
