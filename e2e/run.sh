#!/usr/bin/env bash
#
# End-to-end tests for PKGBUILD repository support (yay.opt.pkgbuild_repos).
#
# Builds yay, then drives real makepkg/pacman installs and upgrades from local
# file:// and git+file:// PKGBUILD repositories, asserting the results. A
# successful install proves the package was resolved from the configured
# repository (the test packages do not exist in the AUR).
#
# Requirements: an Arch Linux environment with makepkg, pacman, git and
# passwordless sudo, run as a NON-root user (makepkg refuses to run as root).
#
# Usage:
#   e2e/run.sh              # builds yay itself
#   YAY_BIN=/path/to/yay e2e/run.sh   # uses an existing binary
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly REPO_ROOT
readonly FIXTURES="${REPO_ROOT}/e2e/fixtures"

GO="${GO:-go}"
YAY_BIN="${YAY_BIN:-}"

fail() {
  echo "E2E FAIL: $*" >&2
  exit 1
}

info() {
  echo "==> $*"
}

# --- preconditions --------------------------------------------------------

[[ ${EUID} -ne 0 ]] || fail "must run as a non-root user (makepkg refuses to run as root)"

for tool in makepkg pacman fakeroot git sudo; do
  command -v "${tool}" >/dev/null 2>&1 || fail "missing required tool: ${tool}"
done

sudo -n true 2>/dev/null || fail "passwordless sudo is required"

# --- workspace ------------------------------------------------------------

WORK="$(mktemp -d)"
readonly WORK
INSTALLED=()

cleanup() {
  if [[ ${#INSTALLED[@]} -gt 0 ]]; then
    sudo pacman -R --noconfirm "${INSTALLED[@]}" >/dev/null 2>&1 || true
  fi
  rm -rf "${WORK}"
}
trap cleanup EXIT

if [[ -z "${YAY_BIN}" ]]; then
  info "building yay"
  YAY_BIN="${WORK}/yay"
  (cd "${REPO_ROOT}" && "${GO}" build -o "${YAY_BIN}" .)
fi
readonly YAY_BIN

# Isolate config and cache so the host environment is untouched.
export XDG_CONFIG_HOME="${WORK}/config"
export XDG_CACHE_HOME="${WORK}/cache"
mkdir -p "${XDG_CONFIG_HOME}/yay"

# --- helpers --------------------------------------------------------------

# make_pkg <repo_dir> <pkgname> <pkgver>
# Instantiates the fixture PKGBUILD into repo_dir/pkgname with a fresh .SRCINFO,
# clearing any previous build artifacts so upgrades rebuild cleanly.
make_pkg() {
  local repo_dir="$1" name="$2" version="$3"
  local pkg_dir="${repo_dir}/${name}"

  mkdir -p "${pkg_dir}"
  rm -rf "${pkg_dir}"/pkg "${pkg_dir}"/src "${pkg_dir}"/*.pkg.tar* 2>/dev/null || true
  sed -e "s/@PKGNAME@/${name}/g" -e "s/@PKGVER@/${version}/g" \
    "${FIXTURES}/PKGBUILD.in" >"${pkg_dir}/PKGBUILD"
  (cd "${pkg_dir}" && makepkg --printsrcinfo >.SRCINFO)
}

# write_config <repo_name> <url>
write_config() {
  cat >"${XDG_CONFIG_HOME}/yay/init.lua" <<EOF
yay.opt.clean_menu = false
yay.opt.diff_menu = false
yay.opt.edit_menu = false
yay.opt.pkgbuild_repos = { ["$1"] = { url = "$2" } }
EOF
}

# assert_version <pkgname> <expected version-rel>
assert_version() {
  local name="$1" want="$2" got
  got="$(pacman -Q "${name}" 2>/dev/null | awk '{print $2}')" || fail "${name} is not installed"
  [[ "${got}" == "${want}" ]] || fail "${name}: expected version ${want}, got ${got}"
  info "OK: ${name} ${got}"
}

# --- scenario 1: install from a file:// local directory repository --------

info "scenario 1: install from a file:// local directory repository"
FILE_REPO="${WORK}/file-repo"
make_pkg "${FILE_REPO}" "yay-e2e-file" "1"
write_config "file-repo" "file://${FILE_REPO}"

sudo pacman -R --noconfirm yay-e2e-file >/dev/null 2>&1 || true
"${YAY_BIN}" -S yay-e2e-file --noconfirm
INSTALLED+=("yay-e2e-file")
assert_version yay-e2e-file "1-1"

# --- scenario 2: upgrade a repo package with -Sua -------------------------

info "scenario 2: upgrade a PKGBUILD-repo package with -Sua"
make_pkg "${FILE_REPO}" "yay-e2e-file" "2"
"${YAY_BIN}" -Sua --noconfirm
assert_version yay-e2e-file "2-1"

# --- scenario 3: install from a git+file:// repository --------------------

info "scenario 3: install from a git+file:// repository"
GIT_REPO="${WORK}/git-repo"
make_pkg "${GIT_REPO}" "yay-e2e-git" "1"
git -C "${GIT_REPO}" init -q
git -C "${GIT_REPO}" config user.email "e2e@example.invalid"
git -C "${GIT_REPO}" config user.name "yay e2e"
git -C "${GIT_REPO}" config commit.gpgsign false
git -C "${GIT_REPO}" add -A
git -C "${GIT_REPO}" commit -qm "init"

write_config "git-repo" "git+file://${GIT_REPO}"

sudo pacman -R --noconfirm yay-e2e-git >/dev/null 2>&1 || true
"${YAY_BIN}" -S yay-e2e-git --noconfirm
INSTALLED+=("yay-e2e-git")
assert_version yay-e2e-git "1-1"

[[ -d "${XDG_CACHE_HOME}/yay/.pkgbuild-repos/git-repo/.git" ]] ||
  fail "git repository was not cloned into the cache"
info "OK: git repository cloned into cache"

echo "E2E PASS: all PKGBUILD repository scenarios succeeded"
