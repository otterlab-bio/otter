#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd -- "${SCRIPT_DIR}/.." && pwd)
BIN=${BIN:-"${REPO_ROOT}/enva/target/debug/enva"}
WORKDIR=$(mktemp -d /tmp/enva-e2e-gha-XXXXXX)
ENV_NAME="enva-e2e-gha"
SECOND_ENV_NAME="enva-e2e-gha-extra"
DUP_ENV_NAME="enva-e2e-gha-dup"
ROOT_PREFIX="${WORKDIR}/root"
EXTERNAL_ROOT_PREFIX="${WORKDIR}/external-root"
YAML_PATH="${WORKDIR}/minimal.yaml"

cleanup() {
  set +e
  rm -rf "${WORKDIR}"
}
trap cleanup EXIT

mkdir -p "${ROOT_PREFIX}" "${EXTERNAL_ROOT_PREFIX}"
cat > "${YAML_PATH}" <<'YAML'
name: ignored-by-cli
channels:
  - conda-forge
dependencies:
  - xz
YAML

export ENVA_RATTLER_ROOT_PREFIX="${ROOT_PREFIX}"
export MAMBA_ROOT_PREFIX="${EXTERNAL_ROOT_PREFIX}"
export ENVA_PACKAGE_MANAGER=micromamba

PREFIX="${ROOT_PREFIX}/envs/${ENV_NAME}"
SECOND_PREFIX="${ROOT_PREFIX}/envs/${SECOND_ENV_NAME}"
DUP_PREFIX="${ROOT_PREFIX}/envs/${DUP_ENV_NAME}"
EXTERNAL_DUP_PREFIX="${EXTERNAL_ROOT_PREFIX}/envs/${DUP_ENV_NAME}"

MICROMAMBA_BIN=${MICROMAMBA_BIN:-}
if [ -z "${MICROMAMBA_BIN}" ] && command -v micromamba >/dev/null 2>&1; then
  MICROMAMBA_BIN=$(command -v micromamba)
fi
if [ -z "${MICROMAMBA_BIN}" ]; then
  echo "micromamba is required for compatibility-layer remove e2e; install it or set MICROMAMBA_BIN" >&2
  exit 1
fi

"${MICROMAMBA_BIN}" --version >/dev/null

"${BIN}" --version
"${BIN}" create --yaml "${YAML_PATH}" --name "${ENV_NAME}" --with jq
"${BIN}" create --yaml "${YAML_PATH}" --name "${SECOND_ENV_NAME}"
"${BIN}" create --yaml "${YAML_PATH}" --name "${DUP_ENV_NAME}"
"${MICROMAMBA_BIN}" create -y -r "${EXTERNAL_ROOT_PREFIX}" -n "${DUP_ENV_NAME}" -c conda-forge xz >/dev/null

test -d "${PREFIX}/conda-meta"
test -d "${SECOND_PREFIX}/conda-meta"
test -d "${DUP_PREFIX}/conda-meta"
test -d "${EXTERNAL_DUP_PREFIX}/conda-meta"

XZ_OUTPUT=$("${BIN}" run "${ENV_NAME}" -- xz --version 2>&1)
printf '%s\n' "${XZ_OUTPUT}"
grep -q "XZ Utils" <<<"${XZ_OUTPUT}"

JQ_OUTPUT=$("${BIN}" run "${ENV_NAME}" -- jq --version 2>&1)
printf '%s\n' "${JQ_OUTPUT}"
grep -q "jq-" <<<"${JQ_OUTPUT}"

BIN="${BIN}" PREFIX="${PREFIX}" ENV_NAME="${ENV_NAME}" bash -lc '
  set -euo pipefail
  PATH_BEFORE="$PATH"
  eval "$($BIN shell hook bash)"
  type enva >/dev/null
  enva activate "$ENV_NAME"
  test "$CONDA_PREFIX" = "$PREFIX"
  test "$ENVA_ACTIVE_NAME" = "$ENV_NAME"
  test "${PATH%%:*}" = "$PREFIX/bin"
  enva deactivate
  test -z "${CONDA_PREFIX-}"
  test -z "${ENVA_ACTIVE_NAME-}"
  test "$PATH" = "$PATH_BEFORE"
'

set +e
NON_TTY_REMOVE_OUTPUT=$("${BIN}" remove "${DUP_ENV_NAME}" 2>&1)
NON_TTY_REMOVE_STATUS=$?
set -e
printf '%s\n' "${NON_TTY_REMOVE_OUTPUT}"
test "${NON_TTY_REMOVE_STATUS}" -ne 0
grep -q "requires interactive selection in a terminal" <<<"${NON_TTY_REMOVE_OUTPUT}"

INTERACTIVE_REMOVE_OUTPUT=$(BIN="${BIN}" DUP_ENV_NAME="${DUP_ENV_NAME}" EXTERNAL_DUP_PREFIX="${EXTERNAL_DUP_PREFIX}" python3 <<'PYTHON'
import os
import pty
import re
import select
import subprocess
import sys

bin_path = os.environ["BIN"]
env_name = os.environ["DUP_ENV_NAME"]
external_prefix = os.environ["EXTERNAL_DUP_PREFIX"]
proc_env = os.environ.copy()
master_fd, slave_fd = pty.openpty()
proc = subprocess.Popen(
    [bin_path, "remove", env_name],
    stdin=slave_fd,
    stdout=slave_fd,
    stderr=slave_fd,
    env=proc_env,
    close_fds=True,
)
os.close(slave_fd)
chunks = []
sent = False
selection = "2\n"
while True:
    if proc.poll() is not None:
        ready, _, _ = select.select([master_fd], [], [], 0.2)
        if not ready:
            break
    else:
        ready, _, _ = select.select([master_fd], [], [], 0.2)
    if not ready:
        continue
    try:
        data = os.read(master_fd, 4096)
    except OSError:
        break
    if not data:
        break
    text = data.decode(errors="replace")
    chunks.append(text)
    if not sent and "Selection for" in "".join(chunks):
        for line in "".join(chunks).splitlines():
            if external_prefix in line:
                match = re.match(r"\s*(\d+)\.", line)
                if match:
                    selection = match.group(1) + "\n"
                    break
        os.write(master_fd, selection.encode())
        sent = True
return_code = proc.wait()
os.close(master_fd)
sys.stdout.write("".join(chunks))
sys.exit(return_code)
PYTHON
)
printf '%s\n' "${INTERACTIVE_REMOVE_OUTPUT}"
grep -q "matched multiple accessible prefixes" <<<"${INTERACTIVE_REMOVE_OUTPUT}"
grep -q "${EXTERNAL_DUP_PREFIX}" <<<"${INTERACTIVE_REMOVE_OUTPUT}"
test -d "${DUP_PREFIX}/conda-meta"
test ! -d "${EXTERNAL_DUP_PREFIX}"

"${BIN}" remove "${ENV_NAME},${SECOND_ENV_NAME}" "${DUP_ENV_NAME}"

test ! -d "${PREFIX}"
test ! -d "${SECOND_PREFIX}"
test ! -d "${DUP_PREFIX}"

echo "enva e2e passed"
