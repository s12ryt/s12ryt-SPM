#!/usr/bin/env bash
# SPM Release installer. Functions are sourceable for isolated behavior tests.
install_root=/

fail() { printf '錯誤：%s\n' "$*" >&2; exit 1; }

fetch() {
    curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
        --connect-timeout 10 --max-time 180 --retry 2 "$@"
}

setting() {
    local key=$1 label=$2 secret=${3:-false} value=${!1:-}
    if [[ -z "$value" ]]; then
        [[ -t 0 ]] || fail "請以環境變數設定 $key，或在互動終端執行。"
        if [[ "$secret" == true ]]; then
            read -r -s -p "$label: " value
            printf '\n' >&2
        else
            read -r -p "$label: " value
        fi
    fi
    [[ -n "$value" && "$value" != *$'\n'* && "$value" != *$'\r'* ]] || fail "$key 不得空白或包含換行。"
    printf -v "$key" '%s' "$value"
}

# systemd EnvironmentFile uses quoting, not shell evaluation. Never source it.
write_setting() {
    local value=$2
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    printf '%s="%s"\n' "$1" "$value"
}

main() (
    set -euo pipefail
    export LC_ALL=C
    local_role=server
    version=''
    if [[ ${1:-} == agent || ${1:-} == server ]]; then local_role=$1; shift; fi
    while (($#)); do
        case "$1" in
            --version) [[ $# -ge 2 ]] || fail '--version 需要版本'; version=$2; shift 2 ;;
            --help|-h) printf '用法：bash install.sh [agent|server] [--version vX.Y.Z]\n預設只安裝 Server；agent 只安裝 Agent。需要 Linux、systemd、root。\n'; exit 0 ;;
            *) fail "不支援的參數：$1" ;;
        esac
    done
    [[ -z "$version" || "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail '版本格式必須為 vX.Y.Z。'
    [[ $(uname -s) == Linux ]] || fail '只支援 Linux VPS。'
    case $(uname -m) in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) fail '只支援 amd64 或 arm64。' ;;
    esac
    [[ $(id -u) == 0 ]] || fail '請以 root 執行；可先 sudo -i 再執行安裝指令。'
    for tool in curl sha256sum systemctl install mktemp awk grep getent useradd chown cp mv; do
        command -v "$tool" >/dev/null || fail "缺少 $tool，請先用系統套件管理員安裝。"
    done
    systemctl show-environment >/dev/null || fail '需要已啟動的 systemd。'
    root=${install_root%/}
    config="$root/etc/spm/$local_role.env"
    destination="$root/opt/spm/spm-$local_role"
    unit="$root/etc/systemd/system/spm-$local_role.service"
    work=$(mktemp -d)
    trap 'rm -rf -- "$work"' EXIT
    umask 077

    if [[ ! -f "$config" ]]; then
        if [[ "$local_role" == server ]]; then
            SPM_ADMIN_USER=${SPM_ADMIN_USER:-admin}
            setting SPM_ADMIN_PASSWORD '管理員密碼（12–72 bytes）' true
            [[ ${#SPM_ADMIN_PASSWORD} -ge 12 && ${#SPM_ADMIN_PASSWORD} -le 72 ]] || fail '管理員密碼必須為 12–72 bytes。'
            setting SPM_ADMIN_USER '管理員帳號'
            SPM_LISTEN=${SPM_LISTEN:-0.0.0.0:8080}
            setting SPM_LISTEN '監聽位址'
            {
                write_setting SPM_ADMIN_USER "$SPM_ADMIN_USER"
                write_setting SPM_ADMIN_PASSWORD "$SPM_ADMIN_PASSWORD"
                write_setting SPM_LISTEN "$SPM_LISTEN"
                write_setting SPM_DATA_DIR '/var/lib/spm'
            } > "$work/config"
        else
            setting SPM_SERVER '主程式 URL'
            setting SPM_TOKEN 'Agent Token' true
            [[ "$SPM_SERVER" =~ ^https?://[^/[:space:]]+(/[^[:space:]]*)?$ && "$SPM_SERVER" != *['?#@']* ]] || fail '主程式 URL 必須為 http(s)，不能包含帳密、query 或 fragment。'
            [[ -z ${SPM_NODE_ID:-} || "$SPM_NODE_ID" =~ ^[a-zA-Z0-9_-]+$ ]] || fail '舊版主機 ID 格式無效。'
            {
                write_setting SPM_SERVER "$SPM_SERVER"
                if [[ -n ${SPM_NODE_ID:-} ]]; then write_setting SPM_NODE_ID "$SPM_NODE_ID"; fi
                write_setting SPM_TOKEN "$SPM_TOKEN"
            } > "$work/config"
        fi
    fi

    repository='https://github.com/s12ryt/s12ryt-SPM'
    if [[ -z "$version" ]]; then
        resolved=$(fetch --output /dev/null --write-out '%{url_effective}' "$repository/releases/latest") || fail '無法取得最新正式 Release。'
        [[ "$resolved" == "$repository/releases/tag/"* ]] || fail '最新 Release 連結無效。'
        version=${resolved##*/}
        [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail '找不到正式版本。'
    fi
    if [[ "$local_role" == agent && "$version" == v0.1.0 ]]; then
        effective_config=$config
        [[ -f "$effective_config" ]] || effective_config="$work/config"
        grep -Eq '^SPM_NODE_ID=("[a-zA-Z0-9_-]+"|[a-zA-Z0-9_-]+)$' "$effective_config" ||
            fail 'v0.1.0 不支援免 ID 接入；請使用支援此功能的新版 Server 與 Agent Release。未變更既有安裝。'
    fi
    asset="spm-$local_role-linux-$arch"
    base="$repository/releases/download/$version"
    printf '下載 %s（%s）…\n' "$asset" "$version"
    fetch --output "$work/$asset" "$base/$asset" || fail 'Release 執行檔下載失敗，未變更既有安裝。'
    fetch --output "$work/SHA256SUMS" "$base/SHA256SUMS" || fail 'Release 校驗碼下載失敗，未變更既有安裝。'
    expected=$(awk -v name="$asset" '$2 == name {print $1}' "$work/SHA256SUMS")
    [[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail '找不到唯一且有效的 SHA-256 校驗碼。'
    actual=$(sha256sum "$work/$asset"); actual=${actual%% *}
    [[ "$actual" == "$expected" ]] || fail 'SHA-256 校驗失敗，未變更既有安裝。'

    if ! getent passwd spm >/dev/null; then
        useradd --system --user-group --home-dir /var/lib/spm --no-create-home --shell /usr/sbin/nologin spm
    fi
    install -d -m 0755 "$root/opt/spm" "$root/etc/systemd/system"
    install -d -m 0700 "$root/etc/spm"
    if [[ "$local_role" == server ]]; then
        install -d -m 0700 "$root/var/lib/spm"
        chown spm:spm "$root/var/lib/spm"
    fi
    if [[ ! -f "$config" ]]; then install -m 0600 "$work/config" "$config"; fi
    if [[ -f "$destination" ]]; then cp -p "$destination" "$work/previous"; fi
    if [[ -f "$unit" ]]; then cp -p "$unit" "$work/previous-unit"; fi
    {
        printf '[Unit]\nDescription=SPM %s\nWants=network-online.target\nAfter=network-online.target\n\n' "$local_role"
        printf '[Service]\nType=simple\nUser=spm\nGroup=spm\n'
        if [[ "$local_role" == server ]]; then
            printf 'StateDirectory=spm\nStateDirectoryMode=0700\nWorkingDirectory=/var/lib/spm\n'
        fi
        printf 'EnvironmentFile=/etc/spm/%s.env\nExecStart=/opt/spm/spm-%s\n' "$local_role" "$local_role"
        printf 'Restart=always\nRestartSec=5\nTimeoutStopSec=15\nNoNewPrivileges=true\nUMask=0077\n\n[Install]\nWantedBy=multi-user.target\n'
    } > "$work/unit"
    service="spm-$local_role"
    # EXIT callbacks: ShellCheck cannot follow this path (upstream issue #2542).
    # shellcheck disable=SC2317
    rollback() {
        printf '更新未完成，嘗試還原舊執行檔與服務設定。\n' >&2
        systemctl stop "$service" || true
        if [[ -f "$work/previous-unit" ]]; then
            cp -p "$work/previous-unit" "$unit.new" && mv -f "$unit.new" "$unit"
        else
            rm -f -- "$unit"
        fi
        if [[ -f "$work/previous" ]]; then
            cp -p "$work/previous" "$destination.new" && mv -f "$destination.new" "$destination"
        else
            rm -f -- "$destination"
        fi
        systemctl daemon-reload || true
        [[ ! -f "$work/previous" ]] || systemctl restart "$service" || true
    }
    # shellcheck disable=SC2317
    cleanup() {
        local result=$?
        if [[ "$result" != 0 && "$replacing" == true ]]; then rollback || true; fi
        rm -rf -- "$work"
        exit "$result"
    }
    replacing=true
    trap 'cleanup' EXIT
    install -m 0755 "$work/$asset" "$destination.new"
    mv -f "$destination.new" "$destination"
    install -m 0644 "$work/unit" "$unit"
    started=true
    if ! systemctl daemon-reload || ! systemctl enable "$service" || ! systemctl restart "$service"; then started=false; fi
    if [[ "$started" == true ]]; then
        sleep 2
        systemctl is-active --quiet "$service" || started=false
    fi
    if [[ "$started" != true ]]; then
        fail "服務啟動失敗。請查看 journalctl -u $service。"
    fi
    replacing=false
    printf '已安裝 %s %s；設定：/etc/spm/%s.env\n' "$service" "$version" "$local_role"
    if [[ "$local_role" == server ]]; then
        printf '首次安裝預設：http://VPS_IP:8080（SPM_LISTEN 可調）；對外使用請配置 HTTPS。\n'
    fi
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    set -euo pipefail
    main "$@"
fi
