#!/usr/bin/env bash
# =============================================================================
#  OTTER Installer
#  Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic
#  Reporting Workflow.
#  Downloads pre-built binaries from GitHub Releases and sets up conda envs.
#
#  Usage:
#    bash install.sh [OPTIONS]
#
#  Options:
#    --install-dir PATH   Override binary installation directory
#    --skip-envs          Skip conda environment creation
#    --non-interactive    Use all defaults without prompting
#    --dry-run            Print all actions without executing
#    --version VER        Specify release version (e.g. v0.3.0); default: latest
#    --releases-repo REPO  Override primary GitHub release repo (owner/name)
#    --fallback-releases-repo REPO  Override fallback GitHub release repo (owner/name)
#    --github-proxy URL   Optional GitHub proxy prefix (for public GitHub URLs)
#    --lang LANG          Interface language: en or zh
#    --help               Show this help message
#
#  Environment:
#    GITHUB_TOKEN / GH_TOKEN / GITHUB_PAT        Optional GitHub token for private release downloads
#    GITHUB_RELEASES_REPO           Optional primary release repo override (owner/name)
#    GITHUB_FALLBACK_RELEASES_REPO  Optional fallback release repo override
#    GITHUB_PROXY_PREFIX / OTTER_GITHUB_PROXY  Optional GitHub proxy prefix
#    OTTER_INSTALL_LANG          Optional interface language override (en|zh)
#
#  Interactive behavior:
#    Interactive mode can ask for an optional GitHub proxy prefix. If GitHub
#    access fails and no token is configured, it can also prompt for a hidden
#    token input and retry once for the current session.
# =============================================================================

set -euo pipefail

# ── Top-level configuration ───────────────────────────────────────────────────
# Release assets are centralized on the umbrella repository: stable releases and the
# daily pre-releases both resolve here, so a single repository is the download source.
RELEASES_REPO="${GITHUB_RELEASES_REPO:-otterlab-bio/otter}"
FALLBACK_RELEASES_REPO="${GITHUB_FALLBACK_RELEASES_REPO:-otterlab-bio/otter}"
OTTER_VERSION="latest"
DEFAULT_INSTALL_DIR=""
USER_HOME="${HOME:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENVS_DIR="$SCRIPT_DIR/../inst/envs"
ACTIVE_ENVS_DIR="$ENVS_DIR"
TMP_ENVS_DIR=""
GITHUB_AUTH_TOKEN="${GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_PAT:-}}}"
GITHUB_PROXY_PREFIX="${GITHUB_PROXY_PREFIX:-${OTTER_GITHUB_PROXY:-}}"
INSTALLER_LANG="${OTTER_INSTALL_LANG:-}"
RELEASE_METADATA=""
RELEASE_METADATA_TAG=""
RELEASE_METADATA_REPO=""
ACTIVE_RELEASES_REPO=""

# All 9 tools: binary_name:release_asset_stem:linkage(static|dynamic)
TOOLS=(
  "otter:otter:static"
  "enva:enva:static"
  "xenofilx:xenofilx:static"
  "pairbam:pairbam:static"
  "seq2mat:seq2mat:static"
  "methx:methx:static"
  "qctb:qctb:static"
  "fastqcx:fastqcx:static"
  "matsrun:matsrun:static"
)

# Conda environment yaml files
ENV_FILES=(
  "otter-core.yaml"
  "otter-snakemake.yaml"
  "otter-extra.yaml"
)

# ── Parse arguments ───────────────────────────────────────────────────────────
INSTALL_DIR=""
SKIP_ENVS=false
NON_INTERACTIVE=false
DRY_RUN=false
SHOW_HELP=false
INSTALL_ENVS_CHOICE="all"   # all | core | snakemake | extra
GITHUB_TOKEN_PROMPT_ATTEMPTED=false
BINARY_OVERWRITE_DECISION=""
BINARY_OVERWRITE_PROMPT_SHOWN=false
declare -A DOWNLOAD_RESUME_ELIGIBLE=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)    INSTALL_DIR="$2"; shift 2 ;;
    --skip-envs)      SKIP_ENVS=true;   shift ;;
    --non-interactive) NON_INTERACTIVE=true; shift ;;
    --dry-run)        DRY_RUN=true;     shift ;;
    --version)        OTTER_VERSION="$2"; shift 2 ;;
    --releases-repo)  RELEASES_REPO="$2"; shift 2 ;;
    --fallback-releases-repo) FALLBACK_RELEASES_REPO="$2"; shift 2 ;;
    --github-proxy)   GITHUB_PROXY_PREFIX="$2"; shift 2 ;;
    --lang)           INSTALLER_LANG="$2"; shift 2 ;;
    --help)           SHOW_HELP=true; shift ;;
    *)
      echo "Unknown option: $1"
      echo "Use --help for usage information."
      exit 1
      ;;
  esac
done

# ── Helpers ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
BOLD='\033[1m'; RESET='\033[0m'

log_info()    { echo -e "  ${BOLD}[INFO]${RESET}  $*"; }
log_success() { echo -e "  ${GREEN}✓${RESET} $*"; }
log_warn()    { echo -e "  ${YELLOW}⚠${RESET}  $*"; }
log_error()   { echo -e "  ${RED}✗${RESET}  $*" >&2; }

detect_default_language() {
  case "${LANG:-}" in
    zh*|ZH*) echo "zh" ;;
    *)       echo "en" ;;
  esac
}

normalize_language() {
  case "${1:-}" in
    zh|ZH|zh-cn|zh_CN|zh-TW|zh_TW|cn|CN|中文) echo "zh" ;;
    en|EN|en-us|en_US|english|English) echo "en" ;;
    "") echo "" ;;
    *) return 1 ;;
  esac
}

prompt_for_language_selection() {
  local default_lang default_choice choice
  default_lang="$(detect_default_language)"

  if [ "$NON_INTERACTIVE" = true ]; then
    INSTALLER_LANG="$default_lang"
    return 0
  fi

  echo "Language / 语言"
  echo "  1) English"
  echo "  2) 中文"

  if [ "$default_lang" = "zh" ]; then
    default_choice="2"
  else
    default_choice="1"
  fi

  read -rp "  Choice [$default_choice]: " choice
  choice="${choice:-$default_choice}"

  case "$choice" in
    1|en|EN|english|English) INSTALLER_LANG="en" ;;
    2|zh|ZH|cn|CN|中文)       INSTALLER_LANG="zh" ;;
    *)                       INSTALLER_LANG="$default_lang" ;;
  esac

  echo ""
}

initialize_language() {
  local normalized

  if [ -n "$INSTALLER_LANG" ]; then
    if ! normalized="$(normalize_language "$INSTALLER_LANG")"; then
      echo "Invalid language: $INSTALLER_LANG. Use --lang en or --lang zh." >&2
      exit 1
    fi
    INSTALLER_LANG="$normalized"
  elif [ "$SHOW_HELP" = true ] || [ "$NON_INTERACTIVE" = true ]; then
    INSTALLER_LANG="$(detect_default_language)"
  else
    prompt_for_language_selection
  fi
}

txt() {
  if [ "$INSTALLER_LANG" = "zh" ]; then
    printf '%s' "$2"
  else
    printf '%s' "$1"
  fi
}

print_help() {
  if [ "$INSTALLER_LANG" = "zh" ]; then
    cat <<'EOF'
# =============================================================================
#  otter 安装脚本
#  从 GitHub Releases 下载预编译二进制文件，并配置 conda 环境。
#
#  用法:
#    bash install.sh [选项]
#
#  选项:
#    --install-dir PATH   指定二进制安装目录
#    --skip-envs          跳过 conda 环境创建
#    --non-interactive    不提示交互，全部使用默认值
#    --dry-run            仅打印操作，不实际执行
#    --version VER        指定发布版本（例如 v0.3.0），默认 latest
#    --releases-repo REPO 指定主 GitHub release 仓库（owner/name）
#    --fallback-releases-repo REPO  指定备用 GitHub release 仓库（owner/name）
#    --github-proxy URL   可选 GitHub 代理前缀（用于公开 GitHub 链接）
#    --lang LANG          界面语言：en 或 zh
#    --help               显示本帮助信息
#
#  环境变量:
#    GITHUB_TOKEN / GH_TOKEN / GITHUB_PAT        私有 release 下载使用的 GitHub token
#    GITHUB_RELEASES_REPO           主 release 仓库覆盖（owner/name）
#    GITHUB_FALLBACK_RELEASES_REPO  备用 release 仓库覆盖
#    GITHUB_PROXY_PREFIX / OTTER_GITHUB_PROXY  GitHub 代理前缀覆盖
#    OTTER_INSTALL_LANG          界面语言覆盖（en|zh）
#
#  交互行为:
#    交互模式下会先选择语言，再输入可选的 GitHub 代理前缀；如果 GitHub
#    访问失败且未配置 token，安装器可以提示输入隐藏 token，并自动重试一次。
# =============================================================================
EOF
  else
    cat <<'EOF'
# =============================================================================
#  otter Installer
#  Downloads pre-built binaries from GitHub Releases and sets up conda envs.
#
#  Usage:
#    bash install.sh [OPTIONS]
#
#  Options:
#    --install-dir PATH   Override binary installation directory
#    --skip-envs          Skip conda environment creation
#    --non-interactive    Use all defaults without prompting
#    --dry-run            Print all actions without executing
#    --version VER        Specify release version (e.g. v0.3.0); default: latest
#    --releases-repo REPO Override primary GitHub release repo (owner/name)
#    --fallback-releases-repo REPO Override fallback GitHub release repo (owner/name)
#    --github-proxy URL   Optional GitHub proxy prefix (for public GitHub URLs)
#    --lang LANG          Interface language: en or zh
#    --help               Show this help message
#
#  Environment:
#    GITHUB_TOKEN / GH_TOKEN / GITHUB_PAT        Optional GitHub token for private release downloads
#    GITHUB_RELEASES_REPO           Optional primary release repo override (owner/name)
#    GITHUB_FALLBACK_RELEASES_REPO  Optional fallback release repo override
#    GITHUB_PROXY_PREFIX / OTTER_GITHUB_PROXY  Optional GitHub proxy prefix
#    OTTER_INSTALL_LANG          Optional interface language override (en|zh)
#
#  Interactive behavior:
#    Interactive mode asks for language first, then an optional GitHub proxy prefix.
#    If GitHub access fails and no token is configured, the installer can prompt for
#    a hidden token input and retry once.
# =============================================================================
EOF
  fi
}

print_reference_genome_instructions() {
  if [ "$INSTALLER_LANG" = "zh" ]; then
    cat <<'EOF'

参考基因组托管于 HuggingFace：
  https://huggingface.co/datasets/Genomiclab/xdxtools-genomes

这是 legacy（旧）参考基因组存档：按组装存放 Bismark 与 STAR 的 tar 包
（hg19、hg38、GRCm38、mm39），不含 reference.yaml、manifest 或 checksums，
因此其中的文件无法校验。它服务于 Snakemake 兼容轨道——该轨道从项目
inst/ 目录读取基因组文件。canonical（规范）轨道改为锁定注册表 release
摘要，且从不把基因组复制进项目；详见 docs/reference-registry.md。

步骤：

  1. 安装 huggingface-cli
       pip install huggingface_hub

  2. 下载压缩包并解压到项目目录
       cd <your-project>
       huggingface-cli download Genomiclab/xdxtools-genomes \
         --local-dir ./inst/ --repo-type dataset
       # 随后需将 ./inst/ 下的每个 *.tar.gz 解压

  3. 仅下载人类基因组（RRBS/WGBS 分析）
       huggingface-cli download Genomiclab/xdxtools-genomes \
         --include "hg19_*" --include "hg38_*" \
         --local-dir ./inst/ --repo-type dataset

  或直接访问浏览器下载后解压至项目 inst/ 目录

EOF
  else
    cat <<'EOF'

Reference genomes are hosted on HuggingFace:
  https://huggingface.co/datasets/Genomiclab/xdxtools-genomes

This is the legacy pre-contract archive. It holds per-assembly Bismark and STAR
tarballs (hg19, hg38, GRCm38, mm39) with no reference.yaml, manifest, or
checksums, so nothing taken from it can be verified. It serves the Snakemake
compatibility track, which reads genome files from a project's inst/ directory.
The canonical track locks a registry release digest instead and never copies
genomes into the project; see docs/reference-registry.md.

Steps:

  1. Install huggingface-cli
       pip install huggingface_hub

  2. Download the archives, then extract them into your project
       cd <your-project>
       huggingface-cli download Genomiclab/xdxtools-genomes \
         --local-dir ./inst/ --repo-type dataset
       # each *.tar.gz in ./inst/ must then be extracted

  3. Download only the human genome set (RRBS/WGBS)
       huggingface-cli download Genomiclab/xdxtools-genomes \
         --include "hg19_*" --include "hg38_*" \
         --local-dir ./inst/ --repo-type dataset

  Or download from the browser and extract into your project's inst/ directory.

EOF
  fi
}

print_completion_summary() {
  echo ""
  divider
  echo -e "${GREEN}${BOLD}$(txt "Installation complete!" "安装完成！")${RESET}"
  divider
  echo ""
  echo "$(txt "Please open a new shell, or run:" "请重新打开终端，或执行：")"
  echo ""
  echo "    source ${SHELL_CONFIG}"
  echo ""
  echo "$(txt "Quick start:" "快速开始：")"
  echo ""
  echo "    otter init my_project"
  echo "    otter create --output my_project --fastq /data/fastq --mode RRBS --pdata samples.csv \\"
  echo "      --reference-root <registry> --reference-primary <id@release>"
  echo "    otter config resolve --project my_project/project.yaml --backend local"
  echo "    otter run --config my_project/runs/<run-id>/run.yaml --executor craftmake --phase step1"
  echo ""
  divider
  echo ""
}

curl_github_unauth() {
  local url="$1" proxied_url
  shift

  if [ -n "$GITHUB_PROXY_PREFIX" ]; then
    proxied_url="$(apply_github_proxy "$url")"
    if curl "$@" "$proxied_url"; then
      return 0
    fi
    log_warn "$(txt "GitHub proxy request failed; retrying direct connection" "GitHub 代理请求失败；正在回退到直连")"
  fi

  curl "$@" "$url"
}

github_api_get() {
  local url="$1"
  if [ -n "$GITHUB_AUTH_TOKEN" ]; then
    curl --retry 3 --retry-all-errors --connect-timeout 15 -sf       -H "Accept: application/vnd.github+json"       -H "Authorization: Bearer $GITHUB_AUTH_TOKEN"       -H "X-GitHub-Api-Version: 2022-11-28"       "$url"
  else
    curl_github_unauth "$url" --retry 3 --retry-all-errors --connect-timeout 15 -sf
  fi
}

github_release_download() {
  local url="$1" dest="$2" tmp_dest="${2}.part" status=0

  if [ -f "$tmp_dest" ] && [ -s "$tmp_dest" ] && [ -z "${DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]:-}" ]; then
    log_info "$(txt "Ignoring stale partial download for $(basename "$dest"); restarting from scratch" "忽略 $(basename "$dest") 的历史残留部分下载；将从头重新下载")"
    rm -f "$tmp_dest"
  fi

  if [ -f "$tmp_dest" ] && [ -s "$tmp_dest" ] && [ -n "${DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]:-}" ]; then
    log_info "$(txt "Resuming partial download for $(basename "$dest") ..." "正在续传 $(basename "$dest") 的部分下载 ...")"
    if [ -n "$GITHUB_AUTH_TOKEN" ] && [[ "$url" == https://api.github.com/repos/*/releases/assets/* ]]; then
      if curl --retry 3 --retry-all-errors --connect-timeout 15 -fL -C - --progress-bar         -H "Accept: application/octet-stream"         -H "Authorization: Bearer $GITHUB_AUTH_TOKEN"         -H "X-GitHub-Api-Version: 2022-11-28"         "$url" -o "$tmp_dest"; then
        mv -f "$tmp_dest" "$dest"
        unset "DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]"
        return 0
      else
        status=$?
      fi
    else
      if curl_github_unauth "$url" --retry 3 --retry-all-errors --connect-timeout 15 -fL -C - --progress-bar -o "$tmp_dest"; then
        mv -f "$tmp_dest" "$dest"
        unset "DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]"
        return 0
      else
        status=$?
      fi
    fi

    if [ "$status" -ne 33 ]; then
      DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]=1
      return "$status"
    fi

    log_warn "$(txt "Server does not support resume for $(basename "$dest"); restarting download from scratch" "服务器不支持 $(basename "$dest") 的续传；将从头重新下载")"
    rm -f "$tmp_dest"
    unset "DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]"
  fi

  if [ -n "$GITHUB_AUTH_TOKEN" ] && [[ "$url" == https://api.github.com/repos/*/releases/assets/* ]]; then
    if ! curl --retry 3 --retry-all-errors --connect-timeout 15 -fL --progress-bar       -H "Accept: application/octet-stream"       -H "Authorization: Bearer $GITHUB_AUTH_TOKEN"       -H "X-GitHub-Api-Version: 2022-11-28"       "$url" -o "$tmp_dest"; then
      status=$?
      if [ -f "$tmp_dest" ] && [ -s "$tmp_dest" ]; then
        DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]=1
      fi
      return "$status"
    fi
  else
    if ! curl_github_unauth "$url" --retry 3 --retry-all-errors --connect-timeout 15 -fL --progress-bar -o "$tmp_dest"; then
      status=$?
      if [ -f "$tmp_dest" ] && [ -s "$tmp_dest" ]; then
        DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]=1
      fi
      return "$status"
    fi
  fi

  mv -f "$tmp_dest" "$dest"
  unset "DOWNLOAD_RESUME_ELIGIBLE[$tmp_dest]"
}

resolve_latest_release_tag_via_page() {
  local repo="$1" url html
  url="https://github.com/${repo}/releases"

  html="$(curl_github_unauth "$url" --retry 3 --retry-all-errors --connect-timeout 15 -fsSL)" || return 1
  printf '%s' "$html"     | grep -o "/${repo}/releases/tag/[^\"?]*"     | head -n 1     | sed 's#.*/tag/##'
}

release_tag_exists_via_page() {
  local repo="$1" tag="$2" url effective_url
  url="https://github.com/${repo}/releases/tag/${tag}"

  effective_url="$(curl_github_unauth "$url" --retry 3 --retry-all-errors --connect-timeout 15 -fsSL -o /dev/null -w '%{url_effective}')" || return 1
  case "$effective_url" in
    */releases/tag/${tag}) return 0 ;;
  esac

  return 1
}

resolve_latest_release_tag() {
  local repo="$1" tag=""
  if tag="$(github_api_get "https://api.github.com/repos/${repo}/releases?per_page=20" | awk -F'"' '/"tag_name"/ && tag == "" { tag = $4 } END { if (tag != "") print tag }' 2>/dev/null)" && [ -n "$tag" ]; then
    printf '%s\n' "$tag"
    return 0
  fi
  resolve_latest_release_tag_via_page "$repo"
}

build_release_repo_candidates() {
  local repo existing found
  local -a repos=()

  for repo in "$RELEASES_REPO" "$FALLBACK_RELEASES_REPO"; do
    [ -n "$repo" ] || continue
    found=false
    for existing in "${repos[@]:-}"; do
      if [ "$existing" = "$repo" ]; then
        found=true
        break
      fi
    done
    if [ "$found" = false ]; then
      repos+=("$repo")
    fi
  done

  printf '%s\n' "${repos[@]}"
}

try_get_release_metadata() {
  local repo="$1" tag="$2" metadata

  if [ "$RELEASE_METADATA_REPO" = "$repo" ] && [ "$RELEASE_METADATA_TAG" = "$tag" ] && [ -n "$RELEASE_METADATA" ]; then
    return 0
  fi

  if ! metadata="$(github_api_get "https://api.github.com/repos/${repo}/releases/tags/${tag}")"; then
    return 1
  fi

  RELEASE_METADATA="$metadata"
  RELEASE_METADATA_REPO="$repo"
  RELEASE_METADATA_TAG="$tag"
  return 0
}

get_release_metadata() {
  local repo="$1" tag="$2"
  if ! try_get_release_metadata "$repo" "$tag"; then
    return 1
  fi
  printf '%s\n' "$RELEASE_METADATA"
}

select_release_repo_for_tag() {
  local tag="$1" repo

  while IFS= read -r repo; do
    [ -n "$repo" ] || continue
    if try_get_release_metadata "$repo" "$tag" || release_tag_exists_via_page "$repo" "$tag"; then
      ACTIVE_RELEASES_REPO="$repo"
      return 0
    fi
  done < <(build_release_repo_candidates)

  return 1
}

resolve_release_asset_api_url() {
  local repo="$1" tag="$2" asset_name="$3"
  get_release_metadata "$repo" "$tag" | awk -v asset_name="$asset_name" '
    /"url": "https:\/\/api.github.com\/repos\/.*\/releases\/assets\// {
      asset_url = $0
      sub(/^.*"url": "/, "", asset_url)
      sub(/".*,?$/, "", asset_url)
    }
    /"name": "/ {
      current_name = $0
      sub(/^.*"name": "/, "", current_name)
      sub(/".*,?$/, "", current_name)
      if (current_name == asset_name && found == "") {
        found = asset_url
      }
    }
    END {
      if (found != "") {
        print found
      }
    }
  '
}

prepare_env_files() {
  local yaml_file url asset_api_url

  ACTIVE_ENVS_DIR="$ENVS_DIR"
  if [ -d "$ACTIVE_ENVS_DIR" ]; then
    return 0
  fi

  TMP_ENVS_DIR=$(mktemp -d)
  for yaml_file in "${ENV_FILES[@]}"; do
    url="${BASE_URL}/${yaml_file}"
    if [ -n "$GITHUB_AUTH_TOKEN" ]; then
      asset_api_url="$(resolve_release_asset_api_url "$ACTIVE_RELEASES_REPO" "$OTTER_VERSION" "$yaml_file" || true)"
      if [ -n "$asset_api_url" ]; then
        url="$asset_api_url"
      fi
    fi

    if ! github_release_download "$url" "${TMP_ENVS_DIR}/${yaml_file}"; then
      rm -rf "$TMP_ENVS_DIR"
      TMP_ENVS_DIR=""
      return 1
    fi
  done

  ACTIVE_ENVS_DIR="$TMP_ENVS_DIR"
  return 0
}

cleanup_installer_tmpdirs() {
  if [ -n "$TMP_ENVS_DIR" ] && [ -d "$TMP_ENVS_DIR" ]; then
    rm -rf "$TMP_ENVS_DIR"
  fi
}

trap cleanup_installer_tmpdirs EXIT

run() {
  if [ "$DRY_RUN" = true ]; then
    echo -e "  ${YELLOW}[DRY-RUN]${RESET} $*"
  else
    eval "$@"
  fi
}

ask() {
  # ask <prompt> <default>
  local prompt="$1" default="$2"
  if [ "$NON_INTERACTIVE" = true ]; then
    echo "$default"
    return
  fi
  read -rp "  $prompt [$default]: " answer
  echo "${answer:-$default}"
}

ask_yn() {
  # ask_yn <prompt> <Y|n>
  local prompt="$1" default="$2"
  if [ "$NON_INTERACTIVE" = true ]; then
    [[ "$default" =~ ^[Yy] ]] && return 0 || return 1
  fi
  read -rp "  $prompt (${default}): " answer
  answer="${answer:-$default}"
  [[ "$answer" =~ ^[Yy] ]] && return 0 || return 1
}

confirm_binary_overwrite() {
  local bin_name="$1"

  if [ -n "$BINARY_OVERWRITE_DECISION" ]; then
    [ "$BINARY_OVERWRITE_DECISION" = "overwrite" ]
    return
  fi

  if ask_yn "$(txt "Overwrite existing binaries starting with ${bin_name}? This choice will apply to all existing binaries." "是否从 ${bin_name} 开始覆盖已存在的二进制文件？该选择会应用到所有已存在的二进制文件。")" "Y"; then
    BINARY_OVERWRITE_DECISION="overwrite"
    return 0
  fi

  BINARY_OVERWRITE_DECISION="keep"
  return 1
}

ask_optional() {
  local prompt="$1" default="${2:-}" answer
  if [ "$NON_INTERACTIVE" = true ]; then
    echo "$default"
    return
  fi
  if [ -n "$default" ]; then
    read -rp "  $prompt [$default]: " answer
    echo "${answer:-$default}"
  else
    read -rp "  $prompt: " answer
    echo "$answer"
  fi
}

trim_whitespace() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf "%s" "$value"
}

normalize_github_proxy_prefix() {
  local value
  value="$(trim_whitespace "${1:-}")"
  if [ -z "$value" ]; then
    printf "%s" ""
    return 0
  fi
  case "$value" in
    http://*|https://*) ;;
    *) return 1 ;;
  esac
  case "$value" in
    */) ;;
    *) value="${value}/" ;;
  esac
  printf "%s" "$value"
}

apply_github_proxy() {
  local url="$1"
  if [ -z "$GITHUB_PROXY_PREFIX" ]; then
    printf "%s\n" "$url"
  else
    printf "%s%s\n" "$GITHUB_PROXY_PREFIX" "$url"
  fi
}

initialize_github_proxy_prefix() {
  local normalized
  if ! normalized="$(normalize_github_proxy_prefix "$GITHUB_PROXY_PREFIX")"; then
    log_error "$(txt "Invalid GitHub proxy prefix. Use a full http(s) URL such as https://gh-proxy.org/" "无效的 GitHub 代理前缀。请使用完整的 http(s) URL，例如 https://gh-proxy.org/")"
    exit 1
  fi
  GITHUB_PROXY_PREFIX="$normalized"
}

resolve_user_home() {
  local user_name resolved
  user_name="$(id -un 2>/dev/null || true)"

  if command -v getent >/dev/null 2>&1 && [ -n "$user_name" ]; then
    resolved="$(getent passwd "$user_name" | cut -d: -f6 | head -n 1)"
  fi
  if [ -z "${resolved:-}" ] && [ -n "$user_name" ]; then
    resolved="$(eval "printf '%s' ~$user_name" 2>/dev/null || true)"
  fi
  if [ -z "${resolved:-}" ]; then
    resolved="${HOME:-}"
  fi

  resolved="$(trim_whitespace "$resolved")"
  [ -n "$resolved" ] || return 1
  printf '%s\n' "$resolved"
}

normalize_shell_config_path() {
  local value
  value="$(trim_whitespace "${1:-}")"
  if [ -z "$value" ]; then
    return 1
  fi

  case "$value" in
    ~)
      value="$USER_HOME"
      ;;
    ~/*)
      value="$USER_HOME/${value#~/}"
      ;;
  esac

  printf '%s\n' "$value"
}

initialize_shell_config_path() {
  local normalized parent

  if ! normalized="$(normalize_shell_config_path "$SHELL_CONFIG")"; then
    normalized="$DEFAULT_SHELL_CONFIG"
  fi

  parent="$(dirname "$normalized")"
  if [ ! -d "$parent" ]; then
    log_warn "$(txt "Shell config directory does not exist: $parent; falling back to $DEFAULT_SHELL_CONFIG" "Shell 配置文件目录不存在：$parent；将回退到 $DEFAULT_SHELL_CONFIG")"
    normalized="$DEFAULT_SHELL_CONFIG"
    parent="$(dirname "$normalized")"
  fi

  if [ -e "$normalized" ]; then
    if [ ! -w "$normalized" ]; then
      log_error "$(txt "Shell config file is not writable: $normalized" "Shell 配置文件不可写：$normalized")"
      exit 1
    fi
  elif [ ! -w "$parent" ]; then
    log_error "$(txt "Cannot create shell config file under: $parent" "无法在以下目录创建 Shell 配置文件：$parent")"
    exit 1
  fi

  SHELL_CONFIG="$normalized"
}

resolve_enva_path() {
  if [ "$DRY_RUN" = true ] && [ -n "${INSTALL_DIR:-}" ]; then
    printf '%s\n' "${INSTALL_DIR}/enva"
    return 0
  fi
  if [ -x "${INSTALL_DIR}/enva" ]; then
    printf '%s\n' "${INSTALL_DIR}/enva"
    return 0
  fi
  if command -v enva >/dev/null 2>&1; then
    command -v enva
    return 0
  fi
  if [ -x "${SCRIPT_DIR}/../enva/target/release/enva" ]; then
    printf '%s\n' "${SCRIPT_DIR}/../enva/target/release/enva"
    return 0
  fi
  return 1
}

run_conda_cache_clean() {
  if [ "$DRY_RUN" = true ]; then
    case "$PM" in
      micromamba)
        echo -e "  ${YELLOW}[DRY-RUN]${RESET} micromamba clean --all --yes"
        ;;
      mamba|conda)
        echo -e "  ${YELLOW}[DRY-RUN]${RESET} $PM clean --all -y"
        ;;
    esac
    return 0
  fi

  log_info "$(txt "Cleaning conda package caches before environment creation ..." "正在创建环境前清理 conda 包缓存 ...")"
  case "$PM" in
    micromamba)
      if micromamba clean --all --yes; then
        log_success "$(txt "Conda caches cleaned" "conda 缓存清理完成")"
      else
        log_warn "$(txt "Conda cache cleanup failed; continuing with environment creation" "conda 缓存清理失败；将继续创建环境")"
      fi
      ;;
    mamba|conda)
      if "$PM" clean --all -y; then
        log_success "$(txt "Conda caches cleaned" "conda 缓存清理完成")"
      else
        log_warn "$(txt "Conda cache cleanup failed; continuing with environment creation" "conda 缓存清理失败；将继续创建环境")"
      fi
      ;;
  esac
}

list_env_path_with_pm() {
  local pm="$1"
  local env_name="$2"
  local line parts_count current_name current_prefix

  command -v "$pm" >/dev/null 2>&1 || return 1

  while IFS= read -r line; do
    [ -z "$line" ] && continue
    case "$line" in
      \#*) continue ;;
    esac

    set -- $line
    parts_count=$#
    [ "$parts_count" -lt 2 ] && continue

    current_name="$1"
    if [ "${2:-}" = "*" ]; then
      current_prefix="${3:-}"
    else
      current_prefix="${2:-}"
    fi

    [ -z "$current_prefix" ] && continue
    if [ "$current_name" = "$env_name" ] || [ "$(basename "$current_prefix")" = "$env_name" ]; then
      printf '%s\n' "$current_prefix"
      return 0
    fi
  done < <("$pm" env list 2>/dev/null || true)

  return 1
}

append_unique_path() {
  local value="$1"
  local -n paths_ref="$2"
  [ -n "$value" ] || return 0

  for existing in "${paths_ref[@]}"; do
    if [ "$existing" = "$value" ]; then
      return 0
    fi
  done

  paths_ref+=("$value")
}

resolve_rattler_env_path() {
  local env_name="$1"
  local -a root_candidates=()
  local root prefix enva_bin conda_prefix conda_parent conda_grandparent var_name raw_value split_value old_ifs

  for var_name in ENVA_RATTLER_ROOT_PREFIX RATTLER_ROOT_PREFIX MAMBA_ROOT_PREFIX; do
    raw_value="${!var_name:-}"
    [ -n "$raw_value" ] || continue
    old_ifs="$IFS"
    IFS=':'
    for split_value in $raw_value; do
      append_unique_path "$split_value" root_candidates
    done
    IFS="$old_ifs"
  done

  conda_prefix="${CONDA_PREFIX:-}"
  if [ -n "$conda_prefix" ]; then
    conda_parent="$(dirname "$conda_prefix")"
    if [ "$(basename "$conda_parent")" = "envs" ]; then
      conda_grandparent="$(dirname "$conda_parent")"
      append_unique_path "$conda_grandparent" root_candidates
    else
      append_unique_path "$conda_prefix" root_candidates
    fi
  fi

  append_unique_path "$HOME/.local/share/rattler" root_candidates
  append_unique_path "$HOME/.local/share/mamba" root_candidates
  append_unique_path "$HOME/.conda" root_candidates

  if enva_bin="$(resolve_enva_path 2>/dev/null)" && [ -n "$enva_bin" ]; then
    append_unique_path "$(cd "$(dirname "$enva_bin")/../.." 2>/dev/null && pwd)/share/rattler" root_candidates
  fi

  for root in "${root_candidates[@]}"; do
    [ -n "$root" ] || continue
    if [ "$env_name" = "base" ] && [ -d "$root/conda-meta" ]; then
      printf '%s\n' "$root"
      return 0
    fi

    prefix="$root/envs/$env_name"
    if [ -d "$prefix" ]; then
      printf '%s\n' "$prefix"
      return 0
    fi
  done

  return 1
}

resolve_conda_env_path() {
  local env_name="$1"
  local pm candidate micromamba_bin micromamba_dir

  for pm in "$PM" micromamba mamba conda; do
    [ -n "$pm" ] || continue
    if candidate="$(list_env_path_with_pm "$pm" "$env_name" 2>/dev/null)" && [ -n "$candidate" ]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done

  if candidate="$(resolve_rattler_env_path "$env_name" 2>/dev/null)" && [ -n "$candidate" ]; then
    printf '%s\n' "$candidate"
    return 0
  fi

  if [ -n "${MAMBA_ROOT_PREFIX:-}" ] && [ -d "${MAMBA_ROOT_PREFIX}/envs/${env_name}" ]; then
    printf '%s\n' "${MAMBA_ROOT_PREFIX}/envs/${env_name}"
    return 0
  fi

  if [ -d "$HOME/.local/share/mamba/envs/${env_name}" ]; then
    printf '%s\n' "$HOME/.local/share/mamba/envs/${env_name}"
    return 0
  fi

  if command -v micromamba >/dev/null 2>&1; then
    micromamba_bin="$(command -v micromamba)"
    micromamba_dir="$(cd "$(dirname "$micromamba_bin")" && pwd)"
    for candidate in \
      "$micromamba_dir/../envs/${env_name}" \
      "$micromamba_dir/../share/mamba/envs/${env_name}"; do
      if [ -d "$candidate" ]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done
  fi

  return 1
}

ask_secret() {
  # ask_secret <prompt>
  local prompt="$1"
  if [ "$NON_INTERACTIVE" = true ]; then
    echo ""
    return
  fi
  local answer
  read -rsp "  $prompt: " answer
  echo >&2
  echo "$answer"
}

maybe_prompt_github_token_on_failure() {
  local reason="${1:-$(txt "GitHub access failed." "GitHub 访问失败。")}"

  if [ -n "$GITHUB_AUTH_TOKEN" ] || [ "$NON_INTERACTIVE" = true ] || [ "$GITHUB_TOKEN_PROMPT_ATTEMPTED" = true ]; then
    return 1
  fi

  GITHUB_TOKEN_PROMPT_ATTEMPTED=true
  log_warn "$reason"

  if ask_yn "$(txt "Enter a GitHub token now and retry once" "现在输入 GitHub token 并重试一次")" "Y"; then
    GITHUB_AUTH_TOKEN=$(ask_secret "$(txt "GitHub token (input hidden, used only for this run)" "GitHub token（隐藏输入，仅用于本次安装）")")
    if [ -n "$GITHUB_AUTH_TOKEN" ]; then
      log_success "$(txt "GitHub token captured for this session" "已读取本次会话使用的 GitHub token")"
      return 0
    fi
    log_warn "$(txt "Empty token entered; continuing without authenticated release access" "未输入 token，将继续以未认证方式访问 release")"
  fi

  return 1
}

divider() { echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"; }

initialize_language
initialize_github_proxy_prefix

if [ "$SHOW_HELP" = true ]; then
  print_help
  exit 0
fi

# ── Step 0: Banner ────────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}╔══════════════════════════════════════════╗${RESET}"
echo -e "${BOLD}║        $(printf "%-26s" "$(txt "otter Installer" "otter 安装器")")║${RESET}"
echo -e "${BOLD}║  $(printf "%-38s" "$(txt "Bioinformatics Workflow Manager" "生物信息工作流管理器")")║${RESET}"
echo -e "${BOLD}╚══════════════════════════════════════════╝${RESET}"
echo ""
[ "$DRY_RUN" = true ] && echo -e "${YELLOW}  $(txt "DRY-RUN MODE – no changes will be made" "DRY-RUN 模式：不会执行任何实际修改")${RESET}\n"

# ── Step 1: System dependency check ──────────────────────────────────────────
echo -e "${BOLD}$(txt "Step 1: Checking system dependencies" "Step 1: 检查系统依赖")${RESET}"

# Required: curl
if ! command -v curl &>/dev/null; then
  log_error "$(txt "curl is required but not found. Please install curl and re-run." "未找到 curl，请先安装 curl 后重试。")"
  exit 1
fi
log_success "$(txt "curl found" "已找到 curl")"
if [ -n "$GITHUB_AUTH_TOKEN" ]; then
  log_success "$(txt "GitHub token detected – authenticated release access enabled" "已检测到 GitHub token，启用认证下载")"
fi

# Architecture detection
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *)
    log_error "$(txt "Unsupported architecture: $ARCH" "不支持的架构: $ARCH")"
    exit 1
    ;;
esac
log_success "$(txt "Architecture: $ARCH" "系统架构: $ARCH")"

USER_HOME="$(resolve_user_home 2>/dev/null || printf '%s\n' "${HOME:-}")"
DEFAULT_INSTALL_DIR="${USER_HOME}/.cargo/bin"
if [ -n "${HOME:-}" ] && [ "$USER_HOME" != "$HOME" ]; then
  log_warn "$(txt "HOME is $HOME, but the current user's home resolves to $USER_HOME; installer defaults will use $USER_HOME" "HOME 当前为 $HOME，但当前用户的家目录解析为 $USER_HOME；安装器默认值将使用 $USER_HOME")"
fi

# Shell detection
DETECTED_SHELL="$(basename "${SHELL:-bash}")"
case "$DETECTED_SHELL" in
  zsh)  DEFAULT_SHELL_CONFIG="$USER_HOME/.zshrc"  ;;
  bash) DEFAULT_SHELL_CONFIG="$USER_HOME/.bashrc" ;;
  *)    DEFAULT_SHELL_CONFIG="$USER_HOME/.profile" ;;
esac
log_success "$(txt "Shell: $DETECTED_SHELL → $DEFAULT_SHELL_CONFIG" "Shell: $DETECTED_SHELL → $DEFAULT_SHELL_CONFIG")"

# conda / mamba / micromamba detection
PM=""
HAS_CONDA=false
for pm in mamba micromamba conda; do
  if command -v "$pm" &>/dev/null; then
    PM="$pm"
    HAS_CONDA=true
    log_success "$(txt "Package manager: $pm" "包管理器: $pm")"
    break
  fi
done
if [ "$HAS_CONDA" = false ]; then
  log_warn "$(txt "No conda/mamba/micromamba found – conda environment steps will be skipped" "未找到 conda/mamba/micromamba，将跳过 conda 环境步骤")"
  HDF5_SKIP_REASON="conda_package_manager_unavailable"
  SKIP_ENVS=true
fi

echo ""

# ── Step 2: Interactive configuration ────────────────────────────────────────
echo -e "${BOLD}$(txt "Step 2: Configuration" "Step 2: 配置")${RESET}"

if [ -z "$INSTALL_DIR" ]; then
  INSTALL_DIR=$(ask "$(txt "Installation directory" "安装目录")" "$DEFAULT_INSTALL_DIR")
fi
log_info "$(txt "Binaries will be installed to: $INSTALL_DIR" "二进制文件将安装到: $INSTALL_DIR")"
log_info "$(txt "Primary GitHub releases repo: $RELEASES_REPO" "主 GitHub release 仓库: $RELEASES_REPO")"
if [ -n "$FALLBACK_RELEASES_REPO" ] && [ "$FALLBACK_RELEASES_REPO" != "$RELEASES_REPO" ]; then
  log_info "$(txt "Fallback GitHub releases repo: $FALLBACK_RELEASES_REPO" "备用 GitHub release 仓库: $FALLBACK_RELEASES_REPO")"
fi
if [ "$NON_INTERACTIVE" = false ] && [ -z "$GITHUB_PROXY_PREFIX" ]; then
  GITHUB_PROXY_PREFIX=$(ask_optional "$(txt "GitHub proxy prefix (optional, e.g. https://gh-proxy.org/)" "GitHub 代理前缀（可选，例如 https://gh-proxy.org/）")")
  initialize_github_proxy_prefix
fi
log_info "$(txt "GitHub proxy prefix: ${GITHUB_PROXY_PREFIX:-<none>}" "GitHub 代理前缀: ${GITHUB_PROXY_PREFIX:-<空>}")"
if [ -n "$GITHUB_PROXY_PREFIX" ] && [ -n "$GITHUB_AUTH_TOKEN" ]; then
  log_info "$(txt "Authenticated GitHub API downloads stay direct to avoid leaking your token to the proxy" "带认证的 GitHub API 下载将保持直连，以避免将 token 暴露给代理")"
fi
SHELL_CONFIG=$(ask "$(txt "Shell config file" "Shell 配置文件")" "$DEFAULT_SHELL_CONFIG")
initialize_shell_config_path
log_info "$(txt "Shell config: $SHELL_CONFIG" "Shell 配置文件: $SHELL_CONFIG")"

if [ "$SKIP_ENVS" = false ]; then
  if ask_yn "$(txt "Install conda environments?" "安装 conda 环境？")" "Y"; then
    echo -e "  $(txt "Which environments? [a]ll / [c]ore / [s]nakemake / [e]xtra" "安装哪些环境？[a]全部 / [c]核心 / [s]Snakemake / [e]扩展")"
    if [ "$NON_INTERACTIVE" = false ]; then
      read -rp "  $(txt "Choice [a]: " "选择 [a]: ")" env_choice
    else
      env_choice="a"
    fi
    case "${env_choice:-a}" in
      c|core)       INSTALL_ENVS_CHOICE="core"       ;;
      s|snakemake)  INSTALL_ENVS_CHOICE="snakemake"  ;;
      e|extra)      INSTALL_ENVS_CHOICE="extra"       ;;
      *)            INSTALL_ENVS_CHOICE="all"         ;;
    esac
    log_info "$(txt "Environments to install: $INSTALL_ENVS_CHOICE" "将安装的环境: $INSTALL_ENVS_CHOICE")"
  else
    SKIP_ENVS=true
    HDF5_SKIP_REASON="conda_environment_installation_skipped"
    log_info "$(txt "Skipping conda environments" "跳过 conda 环境安装")"
  fi
fi

echo ""

# ── Step 3: Resolve version and download binaries ────────────────────────────
echo -e "${BOLD}$(txt "Step 3: Downloading binaries" "Step 3: 下载二进制文件")${RESET}"

# Resolve release tag and source repo via GitHub API
ACTIVE_RELEASES_REPO=""
if [ "$OTTER_VERSION" = "latest" ]; then
  log_info "$(txt "Querying GitHub API for latest release..." "正在查询 GitHub API 获取最新版本...")"
  latest_release_tag=""
  while IFS= read -r candidate_repo; do
    [ -n "$candidate_repo" ] || continue
    if latest_release_tag=$(resolve_latest_release_tag "$candidate_repo" 2>/dev/null) && [ -n "$latest_release_tag" ]; then
      ACTIVE_RELEASES_REPO="$candidate_repo"
      break
    fi
  done < <(build_release_repo_candidates)
  if [ -z "$latest_release_tag" ] && maybe_prompt_github_token_on_failure "$(txt "Latest release query failed. For public repos this usually means GitHub API or proxy connectivity issues; private repos may require a GitHub token." "查询最新 release 失败。对于公开 release 仓库，这通常意味着 GitHub API 或代理连接异常；私有 release 仓库才可能需要 GitHub token。")"; then
    while IFS= read -r candidate_repo; do
      [ -n "$candidate_repo" ] || continue
      if latest_release_tag=$(resolve_latest_release_tag "$candidate_repo" 2>/dev/null) && [ -n "$latest_release_tag" ]; then
        ACTIVE_RELEASES_REPO="$candidate_repo"
        break
      fi
    done < <(build_release_repo_candidates)
  fi
  if [ -z "$latest_release_tag" ]; then
    log_error "$(txt "Could not determine latest version from any configured release repo. Check GitHub connectivity and proxy settings first. Tokens are mainly needed for private repos or rate limits, or use --version to specify." "无法从已配置的 release 仓库确定最新版本。请先检查 GitHub 连通性与代理设置。token 主要用于私有仓库或限流场景，或使用 --version 指定版本。")"
    exit 1
  fi
  OTTER_VERSION="$latest_release_tag"
fi

if [ -z "$ACTIVE_RELEASES_REPO" ] && ! select_release_repo_for_tag "$OTTER_VERSION" 2>/dev/null; then
  ACTIVE_RELEASES_REPO=""
fi
if [ -z "$ACTIVE_RELEASES_REPO" ] && maybe_prompt_github_token_on_failure "$(txt "Release lookup failed for the requested version. For public repos this usually means GitHub connectivity or proxy issues; private repos may require a GitHub token." "查询指定版本 release 失败。对于公开 release 仓库，这通常意味着 GitHub 或代理连接异常；私有 release 仓库才可能需要 GitHub token。")"; then
  if ! select_release_repo_for_tag "$OTTER_VERSION" 2>/dev/null; then
    ACTIVE_RELEASES_REPO=""
  fi
fi
if [ -z "$ACTIVE_RELEASES_REPO" ]; then
  log_error "$(txt "Could not find the requested release in any configured release repo. Adjust --releases-repo / --fallback-releases-repo or use a different version." "无法在已配置的 release 仓库中找到指定版本。请调整 --releases-repo / --fallback-releases-repo，或使用其他版本。")"
  exit 1
fi

log_info "$(txt "Version: $OTTER_VERSION" "版本: $OTTER_VERSION")"
log_info "$(txt "Selected release repo: $ACTIVE_RELEASES_REPO" "已选择的 release 仓库: $ACTIVE_RELEASES_REPO")"

run "mkdir -p \"$INSTALL_DIR\""

BASE_URL="https://github.com/${ACTIVE_RELEASES_REPO}/releases/download/${OTTER_VERSION}"

for entry in "${TOOLS[@]}"; do
  IFS=':' read -r bin_name asset_stem linkage <<< "$entry"
  asset="${asset_stem}-linux-${ARCH}"
  if [ "${linkage}" = "static" ]; then
    asset="${asset}-static"
  fi
  dest="${INSTALL_DIR}/${bin_name}"


  url="${BASE_URL}/${asset}"
  if [ -n "$GITHUB_AUTH_TOKEN" ]; then
    asset_api_url="$(resolve_release_asset_api_url "$ACTIVE_RELEASES_REPO" "$OTTER_VERSION" "$asset" || true)"
    if [ -n "$asset_api_url" ]; then
      url="$asset_api_url"
    fi
  fi
  if [ -f "${dest}.part" ] && [ -s "${dest}.part" ]; then
    log_info "$(txt "Resuming partial download for $bin_name ..." "正在续传 $bin_name 的部分下载 ...")"
  elif [ -f "$dest" ]; then
    if [ "$DRY_RUN" = true ]; then
      if [ "$BINARY_OVERWRITE_PROMPT_SHOWN" = false ]; then
        log_info "$(txt "[DRY-RUN] Existing binaries detected; installer would ask once whether to overwrite all existing binaries (default: yes)" "[DRY-RUN] 检测到已存在的二进制文件；安装器会先统一询问是否覆盖所有已存在二进制（默认：是）")"
        BINARY_OVERWRITE_PROMPT_SHOWN=true
      fi
    elif ! confirm_binary_overwrite "$bin_name"; then
      log_info "$(txt "Skipping $bin_name and keeping the existing binary" "跳过 $bin_name，保留现有二进制文件")"
      continue
    fi
    log_info "$(txt "Updating $bin_name (overwriting existing binary) ..." "正在更新 $bin_name（覆盖已有二进制）...")"
  else
    log_info "$(txt "Downloading $bin_name ..." "正在下载 $bin_name ...")"
  fi

  if [ "$DRY_RUN" = true ]; then
    if [ -n "$GITHUB_AUTH_TOKEN" ] && [[ "$url" == https://api.github.com/repos/*/releases/assets/* ]]; then
      printf '  %b[DRY-RUN]%b curl -fL -C - --progress-bar -H "Accept: application/octet-stream" -H "Authorization: Bearer $GITHUB_TOKEN" "%s" -o "%s.part"\n' "$YELLOW" "$RESET" "$url" "$dest"
      printf '  %b[DRY-RUN]%b mv -f "%s.part" "%s"\n' "$YELLOW" "$RESET" "$dest" "$dest"
    else
      printf '  %b[DRY-RUN]%b curl -fL -C - --progress-bar "%s" -o "%s.part" (proxy first, then direct fallback)\n' "$YELLOW" "$RESET" "$(apply_github_proxy "$url")" "$dest"
      printf '  %b[DRY-RUN]%b mv -f "%s.part" "%s"\n' "$YELLOW" "$RESET" "$dest" "$dest"
    fi
  else
    if github_release_download "$url" "$dest"; then
      chmod +x "$dest"
      log_success "$(txt "$bin_name installed" "$bin_name 安装完成")"
    elif maybe_prompt_github_token_on_failure "$(txt "Download failed for $bin_name. Private release assets usually require a GitHub token." "$bin_name 下载失败。私有 release 资产通常需要 GitHub token。")"; then
      if github_release_download "$url" "$dest"; then
        chmod +x "$dest"
        log_success "$(txt "$bin_name installed" "$bin_name 安装完成")"
      else
        log_warn "$(txt "$bin_name download failed (HTTP error – asset may not exist for this release, or authentication is required)" "$bin_name 下载失败（HTTP 错误：该版本可能不存在该资产，或需要认证）")"
      fi
    else
      log_warn "$(txt "$bin_name download failed (HTTP error – asset may not exist for this release, or authentication is required)" "$bin_name 下载失败（HTTP 错误：该版本可能不存在该资产，或需要认证）")"
    fi
  fi
done

echo ""

# ── Step 4: Verify tools ──────────────────────────────────────────────────────
echo -e "${BOLD}$(txt "Step 4: Verifying installed tools" "Step 4: 验证已安装工具")${RESET}"

# Temporarily add INSTALL_DIR to PATH for verification
export PATH="${INSTALL_DIR}:${PATH}"

for entry in "${TOOLS[@]}"; do
  bin_name="${entry%%:*}"
  dest="${INSTALL_DIR}/${bin_name}"

  if [ "$DRY_RUN" = true ]; then
    echo -e "  ${YELLOW}[DRY-RUN]${RESET} $(txt "would verify: $bin_name --version" "将验证：$bin_name --version")"
    continue
  fi

  if [ -f "$dest" ]; then
    ver=$("$dest" --version 2>&1 | head -1 || echo "(version unknown)")
    log_success "$bin_name  $ver"
  else
    log_warn "$(txt "$bin_name not found at $dest" "$bin_name 未在 $dest 找到")"
  fi
done

echo ""

# ── Step 5: Configure PATH ────────────────────────────────────────────────────
echo -e "${BOLD}$(txt "Step 5: Configuring PATH" "Step 5: 配置 PATH")${RESET}"

if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
  log_info "$(txt "Adding $INSTALL_DIR to PATH in $SHELL_CONFIG" "正在将 $INSTALL_DIR 添加到 $SHELL_CONFIG 的 PATH")"
  run "echo '' >> \"$SHELL_CONFIG\""
  run "echo '# otter: binary install directory' >> \"$SHELL_CONFIG\""
  run "echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> \"$SHELL_CONFIG\""
  log_success "$(txt "PATH updated in $SHELL_CONFIG" "已更新 $SHELL_CONFIG 中的 PATH")"
else
  log_success "$(txt "$INSTALL_DIR is already in PATH" "$INSTALL_DIR 已存在于 PATH 中")"
fi

echo ""

# ── Step 6: Create conda environments ────────────────────────────────────────
if [ "$SKIP_ENVS" = false ]; then
  echo -e "${BOLD}$(txt "Step 6: Creating conda environments" "Step 6: 创建 conda 环境")${RESET}"

  if prepare_env_files; then
    if [ "$ACTIVE_ENVS_DIR" != "$ENVS_DIR" ]; then
      log_info "$(txt "Local inst/envs not found; using environment YAMLs downloaded from ${ACTIVE_RELEASES_REPO}" "未找到本地 inst/envs；将使用从 ${ACTIVE_RELEASES_REPO} 下载的环境 YAML")"
    fi
  else
    log_warn "$(txt "Could not access inst/envs locally or download environment YAMLs from ${ACTIVE_RELEASES_REPO}" "无法访问本地 inst/envs，也无法从 ${ACTIVE_RELEASES_REPO} 下载环境 YAML")"
    log_warn "$(txt "Skipping conda environment setup" "跳过 conda 环境安装")"
    SKIP_ENVS=true
  fi
fi

if [ "$SKIP_ENVS" = false ]; then
  enva_bin=""
  if enva_bin="$(resolve_enva_path 2>/dev/null)"; then
    log_info "$(txt "Using enva (rattler-first) for environment creation: $enva_bin" "将使用 enva（rattler 优先）创建环境：$enva_bin")"
    log_info "$(txt "enva will create rattler-managed environments, replacing conflicts and cleaning caches before the first creation" "enva 将创建由 rattler 管理的环境，并在首次创建前处理冲突环境和清理缓存")"
  else
    log_warn "$(txt "enva not found; falling back to $PM env create" "未找到 enva；将回退到 $PM env create")"
    run_conda_cache_clean
  fi

  declare -A ENV_SELECT
  case "$INSTALL_ENVS_CHOICE" in
    all)
      for f in "${ENV_FILES[@]}"; do ENV_SELECT["$f"]=1; done
      ;;
    core)
      ENV_SELECT["otter-core.yaml"]=1
      ;;
    snakemake)
      ENV_SELECT["otter-core.yaml"]=1
      ENV_SELECT["otter-snakemake.yaml"]=1
      ;;
    extra)
      for f in "${ENV_FILES[@]}"; do ENV_SELECT["$f"]=1; done
      ;;
  esac

  enva_clean_cache_pending=true

  for yaml_file in "${ENV_FILES[@]}"; do
    [ -z "${ENV_SELECT[$yaml_file]+x}" ] && continue
    env_name="${yaml_file%.yaml}"
    yaml_path="${ACTIVE_ENVS_DIR}/${yaml_file}"

    if [ ! -f "$yaml_path" ]; then
      log_warn "$(txt "$yaml_file not found, skipping $env_name" "$yaml_file 未找到，跳过 $env_name")"
      continue
    fi

    if [ -n "$enva_bin" ]; then
      log_info "$(txt "Creating environment via enva: $env_name ..." "正在通过 enva 创建环境：$env_name ...")"
      enva_args=(create --yaml "$yaml_path" --name "$env_name" --force)
      if [ "$enva_clean_cache_pending" = true ]; then
        enva_args+=(--clean-cache)
      fi

      if [ "$DRY_RUN" = false ]; then
        if "$enva_bin" "${enva_args[@]}"; then
          log_success "$(txt "$env_name created" "$env_name 创建完成")"
        else
          log_error "$(txt "Failed to create $env_name via enva" "通过 enva 创建 $env_name 失败")"
          exit 1
        fi
      else
        printf '  %b[DRY-RUN]%b %q --dry-run' "$YELLOW" "$RESET" "$enva_bin"
        for arg in "${enva_args[@]}"; do
          printf ' %q' "$arg"
        done
        printf '\n'
      fi

      enva_clean_cache_pending=false
    else
      if [ "$DRY_RUN" = false ]; then
        if $PM env list 2>/dev/null | grep -qE "^${env_name}([[:space:]]|$)"; then
          log_info "$(txt "Replacing existing environment: $env_name ..." "正在覆盖已有环境：$env_name ...")"
          $PM env remove -n "$env_name" -y
        fi
        log_info "$(txt "Creating environment: $env_name ..." "正在创建环境：$env_name ...")"
        $PM env create -f "$yaml_path" -y
        log_success "$(txt "$env_name created" "$env_name 创建完成")"
      else
        echo -e "  ${YELLOW}[DRY-RUN]${RESET} $PM env remove -n \"$env_name\" -y  # $(txt "if the environment already exists" "如果环境已存在")"
        echo -e "  ${YELLOW}[DRY-RUN]${RESET} $PM env create -f \"$yaml_path\" -y"
      fi
    fi
  done
fi
[ "$SKIP_ENVS" = true ] || echo ""

# ── Step 7: Reference genome download instructions ───────────────────────────
echo ""
divider
echo -e "${BOLD}$(txt "Reference Genome Download Guide" "参考基因组下载指引")${RESET}"
divider
print_reference_genome_instructions
divider

# ── Final summary ─────────────────────────────────────────────────────────────
print_completion_summary
