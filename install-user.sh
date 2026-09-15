#!/usr/bin/env bash
# Non-root installer: SPM binaries are downloaded only from verified Releases.
fail() { printf '錯誤：%s\n' "$*" >&2; exit 1; }
fetch() {
    curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
        --connect-timeout 10 --max-time 180 --retry 2 "$@"
}
setting() {
    local key=$1 label=$2 secret=${3:-false} value=${!1:-}
    if [[ -z "$value" ]]; then
        [[ -t 0 ]] || fail "請以環境變數設定 $key，或在互動終端執行。"
        if [[ "$secret" == true ]]; then read -r -s -p "$label: " value; printf '\n' >&2
        else read -r -p "$label: " value; fi
    fi
    [[ -n "$value" && "$value" != *$'\n'* && "$value" != *$'\r'* ]] || fail "$key 不得空白或包含換行。"
    printf -v "$key" '%s' "$value"
}

write_control() {
    cat <<'CONTROL'
#!/usr/bin/env bash
set -euo pipefail
role=${1:-}; action=${2:-}
[[ $# == 2 && "$role" =~ ^(server|agent)$ && "$action" =~ ^(start|stop|restart|status|logs)$ ]] || {
    echo '用法：spm-user server|agent start|stop|restart|status|logs' >&2; exit 2;
}
base="$HOME/.local/share/spm"
dir="$base/$role"
[[ -f "$dir/manager" ]] || { echo '此角色尚未安裝。' >&2; exit 1; }
exec 9>"$dir/control.lock"
flock -w 10 9 || { echo '另一個服務操作仍在執行。' >&2; exit 1; }
manager=$(cat "$dir/manager")
if [[ "$manager" == systemd ]]; then
    if [[ "$action" == logs ]]; then
        timeout 30 journalctl --user -u "spm-$role" -n 100 --no-pager
    else
        timeout 30 systemctl --user "$action" "spm-$role" --no-pager
    fi
    exit
fi
[[ "$manager" == supervisor ]] || { echo '程序管理設定無效。' >&2; exit 1; }
ctl=(timeout 25 "$base/venv/bin/supervisorctl" -c "$dir/supervisord.conf")
if [[ "$action" == logs ]]; then
    if [[ -f "$dir/program.log" ]]; then tail -n 100 "$dir/program.log"; else echo '尚無日誌。'; fi
    exit
fi
if [[ "$action" == status ]]; then "${ctl[@]}" status spm; exit; fi
if [[ "$action" == stop ]]; then
    if "${ctl[@]}" pid >/dev/null 2>&1; then
        "${ctl[@]}" shutdown
        for ((i=0; i<20; i++)); do
            if ! "${ctl[@]}" pid >/dev/null 2>&1; then exit 0; fi
            sleep 1
        done
        echo 'Supervisor 停止逾時。' >&2; exit 1
    fi
    exit 0
fi
if "${ctl[@]}" pid >/dev/null 2>&1; then
    if [[ "$action" == start ]] && "${ctl[@]}" status spm | grep -q ' RUNNING '; then exit 0; fi
    "${ctl[@]}" "$action" spm
else
    timeout 15 "$base/venv/bin/supervisord" -c "$dir/supervisord.conf" 8>&- 9>&-
fi
for ((i=0; i<15; i++)); do
    if "${ctl[@]}" status spm 2>/dev/null | grep -q ' RUNNING '; then exit 0; fi
    sleep 1
done
echo '服務未成功啟動；請執行 spm-user 對應角色 logs。' >&2
exit 1
CONTROL
}

main() (
    set -euo pipefail
    export LC_ALL=C
    local_role=server; version=''
    server_set=false; token_set=false
    if [[ ${1:-} == agent || ${1:-} == server ]]; then local_role=$1; shift; fi
    while (($#)); do
        case "$1" in
            --version) [[ $# -ge 2 ]] || fail '--version 需要版本'; version=$2; shift 2 ;;
            --server|--token)
                option=$1
                [[ $# -ge 2 && "$2" != --* ]] || fail "$option 需要值。"
                value=$2; shift 2
                case "$option" in --server) SPM_SERVER=$value; server_set=true ;; --token) SPM_TOKEN=$value; token_set=true ;; esac ;;
            --server=*) SPM_SERVER=${1#*=}; server_set=true; shift ;;
            --token=*) SPM_TOKEN=${1#*=}; token_set=true; shift ;;
            --help|-h) printf '用法：bash install-user.sh [agent|server] [--version vX.Y.Z]；預設只安裝 Server。\nAgent 參數：--server URL --token TOKEN（明確提供時更新既有設定）\n'; exit 0 ;;
            *) fail '不支援的參數；請查看 --help。' ;;
        esac
    done
    if [[ "$server_set" == true || "$token_set" == true ]]; then
        [[ "$local_role" == agent ]] || fail '--server／--token 僅供 agent 使用。'
    fi
    if [[ "$server_set" == true ]]; then
        [[ "$SPM_SERVER" =~ ^https?://[^/[:space:]]+(/[^[:space:]]*)?$ && "$SPM_SERVER" != *['?#@']* ]] || fail '--server 必須為有效 http(s) URL，不能包含帳密、query 或 fragment。'
    fi
    if [[ "$token_set" == true ]]; then
        [[ -n "$SPM_TOKEN" && "$SPM_TOKEN" != *$'\n'* && "$SPM_TOKEN" != *$'\r'* ]] || fail '--token 不得空白或包含換行。'
    fi
    [[ -z "$version" || "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail '版本格式必須為 vX.Y.Z。'
    [[ $(uname -s) == Linux ]] || fail '只支援 Linux VPS。'
    case $(uname -m) in
        x86_64|amd64) arch=amd64 ;;
        aarch64|arm64) arch=arm64 ;;
        *) fail '只支援 amd64 或 arm64。' ;;
    esac
    [[ $(id -u) != 0 ]] || fail '請以一般使用者執行；root 請使用 install.sh。'
    [[ ${HOME:-} == /* && -d "$HOME" && -w "$HOME" && "$HOME" != *$'\n'* && "$HOME" != *$'\r'* ]] || fail '需要可寫入的絕對 HOME 目錄。'
    for tool in curl sha256sum install mktemp awk cp mv flock timeout grep; do
        command -v "$tool" >/dev/null || fail "缺少 $tool；請使用 VPS 提供的工具環境。"
    done
    base="$HOME/.local/share/spm"
    dir="$base/$local_role"
    destination="$dir/spm-$local_role"
    unit="$HOME/.config/systemd/user/spm-$local_role.service"
    control="$HOME/.local/bin/spm-user"
    umask 077
    install -d -m 0700 "$base"
    exec 8>"$base/install.lock"
    flock -w 10 8 || fail '另一個安裝仍在執行。'
    work=$(mktemp -d)
    trap 'rm -rf -- "$work"' EXIT
    if [[ -f "$dir/manager" ]]; then manager=$(cat "$dir/manager")
    elif command -v systemctl >/dev/null && timeout 5 systemctl --user show-environment >/dev/null 2>&1; then manager=systemd
    else manager=supervisor; fi
    [[ "$manager" == systemd || "$manager" == supervisor ]] || fail '既有程序管理設定無效。'
    config_update=false
    if [[ -f "$dir/config.env" && ( "$server_set" == true || "$token_set" == true ) ]]; then
        {
            while IFS= read -r line || [[ -n "$line" ]]; do
                [[ "$server_set" != true || "$line" != SPM_SERVER=* ]] || continue
                [[ "$token_set" != true || "$line" != SPM_TOKEN=* ]] || continue
                printf '%s\n' "$line"
            done < "$dir/config.env"
            if [[ "$server_set" == true ]]; then printf 'SPM_SERVER=%s\n' "$SPM_SERVER"; fi
            if [[ "$token_set" == true ]]; then printf 'SPM_TOKEN=%s\n' "$SPM_TOKEN"; fi
        } > "$work/config.env"
        config_update=true
    fi

    if [[ ! -f "$dir/config.env" ]]; then
        if [[ "$local_role" == server ]]; then
            SPM_ADMIN_USER=${SPM_ADMIN_USER:-admin}
            setting SPM_ADMIN_PASSWORD '管理員密碼（12–72 bytes）' true
            [[ ${#SPM_ADMIN_PASSWORD} -ge 12 && ${#SPM_ADMIN_PASSWORD} -le 72 ]] || fail '管理員密碼必須為 12–72 bytes。'
            setting SPM_ADMIN_USER '管理員帳號'
            SPM_LISTEN=${SPM_LISTEN:-0.0.0.0:8080}
            setting SPM_LISTEN '監聽位址（一般帳號請使用大於 1023 的 port）'
            printf 'SPM_ADMIN_USER=%s\nSPM_ADMIN_PASSWORD=%s\nSPM_LISTEN=%s\nSPM_DATA_DIR=%s\n' \
                "$SPM_ADMIN_USER" "$SPM_ADMIN_PASSWORD" "$SPM_LISTEN" "$dir/data" > "$work/config.env"
        else
            setting SPM_SERVER '主程式 URL'; setting SPM_TOKEN 'Agent Token' true
            [[ "$SPM_SERVER" =~ ^https?://[^/[:space:]]+(/[^[:space:]]*)?$ && "$SPM_SERVER" != *['?#@']* ]] || fail '主程式 URL 必須為 http(s)，不能包含帳密、query 或 fragment。'
            [[ -z ${SPM_NODE_ID:-} || "$SPM_NODE_ID" =~ ^[a-zA-Z0-9_-]+$ ]] || fail '舊版主機 ID 格式無效。'
            {
                printf 'SPM_SERVER=%s\nSPM_TOKEN=%s\n' "$SPM_SERVER" "$SPM_TOKEN"
                if [[ -n ${SPM_NODE_ID:-} ]]; then printf 'SPM_NODE_ID=%s\n' "$SPM_NODE_ID"; fi
            } > "$work/config.env"
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
        effective_config="$dir/config.env"
        [[ ! -f "$work/config.env" ]] || effective_config="$work/config.env"
        grep -Eq '^SPM_NODE_ID=[a-zA-Z0-9_-]+$' "$effective_config" ||
            fail 'v0.1.0 不支援免 ID 接入；請使用支援此功能的新版 Server 與 Agent Release。未變更既有安裝。'
    fi
    asset="spm-$local_role-linux-$arch"
    printf '下載 %s（%s）…\n' "$asset" "$version"
    fetch --output "$work/binary" "$repository/releases/download/$version/$asset" || fail 'Release 下載失敗，未變更既有安裝。'
    fetch --output "$work/SHA256SUMS" "$repository/releases/download/$version/SHA256SUMS" || fail 'Release 校驗碼下載失敗。'
    expected=$(awk -v name="$asset" '$2 == name {print $1}' "$work/SHA256SUMS")
    [[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail '找不到唯一有效的 SHA-256 校驗碼。'
    actual=$(sha256sum "$work/binary"); actual=${actual%% *}
    [[ "$actual" == "$expected" ]] || fail 'SHA-256 校驗失敗，未變更既有安裝。'

    if [[ "$manager" == supervisor ]]; then
        [[ ${#dir} -lt 88 ]] || fail 'HOME 過長，超過 Supervisor UNIX socket 路徑限制。'
        if [[ ! -x "$base/venv/bin/supervisord" ]]; then
            command -v python3 >/dev/null || fail '需要 Python 3 與 venv 才能安裝 Supervisor。'
            timeout 60 python3 -m venv "$base/venv" || fail 'Python venv 不可用；請洽 VPS 業者提供 Python 3 venv。'
            timeout 120 "$base/venv/bin/python" -m pip install --disable-pip-version-check --no-input \
                --only-binary=:all: 'supervisor==4.3.0' || fail '使用者 Supervisor 安裝失敗。'
        fi
    fi
    install -d -m 0700 "$dir" "$HOME/.local/bin"
    [[ "$local_role" != server ]] || install -d -m 0700 "$dir/data"
    [[ -f "$dir/config.env" ]] || install -m 0600 "$work/config.env" "$dir/config.env"
    # Config is literal KEY=value data, never sourced or evaluated as shell code.
    cat > "$work/run" <<'RUN'
#!/usr/bin/env bash
set -euo pipefail
dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
role=${dir##*/}
unset DATABASE_URL SPM_ADMIN_USER SPM_ADMIN_PASSWORD SPM_LISTEN SPM_DATA_DIR SPM_SERVER SPM_NODE_ID SPM_TOKEN
while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" =~ ^(SPM_[A-Z_]+|DATABASE_URL)= ]] || { echo 'config.env 格式無效。' >&2; exit 1; }
    export "$line"
done < "$dir/config.env"
exec "$dir/spm-$role"
RUN
    write_control > "$work/control"
    if [[ "$manager" == systemd ]]; then
        install -d -m 0700 "$HOME/.config/systemd/user"
        {
            printf '[Unit]\nDescription=SPM %s (user)\n\n[Service]\n' "$local_role"
            printf 'ExecStart=/bin/bash "%%h/.local/share/spm/%s/run"\n' "$local_role"
            printf 'Restart=always\nRestartSec=5\nTimeoutStopSec=15\nNoNewPrivileges=true\nUMask=0077\n\n[Install]\nWantedBy=default.target\n'
        } > "$work/service"
        service_file="$unit"
    else
        cat > "$work/service" <<'SUPERVISOR'
[unix_http_server]
file=%(here)s/control.sock
chmod=0600
[supervisord]
logfile=%(here)s/supervisor.log
logfile_maxbytes=1MB
logfile_backups=2
pidfile=%(here)s/supervisor.pid
directory=%(here)s
umask=077
minfds=128
minprocs=32
[rpcinterface:supervisor]
supervisor.rpcinterface_factory=supervisor.rpcinterface:make_main_rpcinterface
[supervisorctl]
serverurl=unix://%(here)s/control.sock
[program:spm]
command=/bin/bash "%(here)s/run"
autostart=true
autorestart=true
startsecs=2
startretries=3
stopwaitsecs=15
stopasgroup=true
killasgroup=true
redirect_stderr=true
stdout_logfile=%(here)s/program.log
stdout_logfile_maxbytes=5MB
stdout_logfile_backups=2
SUPERVISOR
        service_file="$dir/supervisord.conf"
    fi
    for file in "$destination" "$service_file" "$dir/run" "$control"; do
        [[ ! -f "$file" ]] || cp -p "$file" "$work/old-$(basename "$file")"
    done
    if [[ "$config_update" == true ]]; then cp -p "$dir/config.env" "$work/previous-config"; fi
    # EXIT callbacks: ShellCheck cannot follow this path (upstream issue #2542).
    # shellcheck disable=SC2317
    rollback() {
        printf '更新未完成，嘗試還原舊執行檔與服務設定。\n' >&2
        if [[ -f "$control" && -f "$dir/manager" ]]; then
            "$control" "$local_role" stop >/dev/null 2>&1 || true
        fi
        if [[ -f "$work/previous-config" ]]; then
            cp -p "$work/previous-config" "$dir/config.env.new" && mv -f "$dir/config.env.new" "$dir/config.env"
        fi
        for file in "$destination" "$service_file" "$dir/run" "$control"; do
            if [[ -f "$work/old-$(basename "$file")" ]]; then
                cp -p "$work/old-$(basename "$file")" "$file.new" && mv -f "$file.new" "$file"
            else
                rm -f -- "$file"
            fi
        done
        [[ "$manager" != systemd ]] || timeout 30 systemctl --user daemon-reload || true
        [[ ! -f "$work/old-spm-$local_role" ]] || "$control" "$local_role" start || true
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
    install -m 0700 "$work/binary" "$destination.new"; mv -f "$destination.new" "$destination"
    if [[ "$config_update" == true ]]; then
        install -m 0600 "$work/config.env" "$dir/config.env.new"; mv -f "$dir/config.env.new" "$dir/config.env"
    fi
    install -m 0700 "$work/run" "$dir/run"
    install -m 0700 "$work/control" "$control"
    install -m 0600 "$work/service" "$service_file"
    printf '%s\n' "$manager" > "$dir/manager"
    started=true
    if [[ "$manager" == systemd ]]; then
        if ! timeout 30 systemctl --user daemon-reload || ! timeout 30 systemctl --user enable "spm-$local_role"; then started=false; fi
    fi
    if [[ "$started" == true ]]; then
        "$control" "$local_role" restart || started=false
        sleep 2
        if [[ "$manager" == systemd ]]; then timeout 10 systemctl --user is-active --quiet "spm-$local_role" || started=false
        else "$control" "$local_role" status >/dev/null || started=false; fi
    fi
    if [[ "$started" != true ]]; then
        fail "服務啟動失敗。請查看：$control $local_role logs"
    fi
    replacing=false
    printf '已安裝 %s %s（%s）；設定：%s/config.env\n' "$local_role" "$version" "$manager" "$dir"
    printf '管理指令：%s %s start|stop|restart|status|logs\n' "$control" "$local_role"
    if [[ "$manager" == supervisor ]]; then
        echo 'Supervisor 已於背景執行；不會自動設定開機啟動，重開機後請執行 start。登出存活取決於業者的工作階段政策。'
    elif ! command -v loginctl >/dev/null || [[ $(timeout 5 loginctl show-user "$(id -u)" -p Linger --value 2>/dev/null || true) != yes ]]; then
        echo '尚未確認 linger；已啟用使用者服務，但登出後或開機時持續執行需業者允許 linger。'
    fi
    [[ "$local_role" != server ]] || echo '首次預設面板：http://VPS_IP:8080；SPM_LISTEN 可調整，請使用業者允許的 port。'
)

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
