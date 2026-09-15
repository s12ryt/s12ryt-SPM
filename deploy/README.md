# Linux 部署範例

建議使用 [README 一鍵安裝](../README.md#vps-一鍵安裝)，從 Releases 取得已建置程式並建立 systemd 服務。下列為手動安裝替代步驟；一鍵安裝預設 Server 監聽 `0.0.0.0:8080`，手動範本則為 `127.0.0.1:8080`。

沒有 root 權限時使用 [免 root 一鍵安裝](../README.md#沒有-root-的-vps)。它只寫入自己的家目錄，優先使用 `systemd --user`，否則以 Python venv 安裝 Supervisor；用 `~/.local/bin/spm-user server start|stop|restart|status|logs` 管理。請使用業者允許的非特權 port。設定是 `~/.local/share/spm/server/config.env` 的純文字 `KEY=值`；備份前先停止 Server，再複製整個 `server/data`。Supervisor 不自動設定重開機啟動，登出存活仍受業者政策限制。

兩種免 root 管理方式皆已於 Ubuntu 一般帳號實測，包含崩潰重啟與更新保留：[驗收證據](../agent/user-install-validation.md)。

以下操作由管理系統的人員在目標 VPS 執行。採一般 `spm` 帳號執行主程式與 Agent；收集一般主機資訊不需要 root。

Agent 範本只填 `SPM_SERVER` 與 `SPM_TOKEN`，ID 由 Server 管理。免 ID 接入已於正式 [v0.1.1](https://github.com/s12ryt/s12ryt-SPM/releases/tag/v0.1.1) 提供；先更新 Server，再更新 Agent，既有含 ID 的設定可保留。詳見 [版本要求](../README.md#vps-一鍵安裝)與[正式免 ID 安裝驗收](../agent/token-enrollment-validation.md)。

兩種一鍵安裝均可在 `agent` 後加 `--server 'https://monitor.example.com' --token '你的Token'`。首次安裝參數優先於環境變數；更新時只替換明確指定的連線欄位，未給參數則保留原設定，更新失敗會嘗試回復。直接啟動 Agent 的同名參數使用方式與版本要求見 [快速啟動](../README.md#快速啟動)；正式 v0.1.1 的 Token 仍由環境變數提供，安裝腳本會代為保存至設定檔。

1. 建立專用 `spm` 使用者，將對應 CPU 架構的執行檔安裝成 `/opt/spm/spm-server` 或 `/opt/spm/spm-agent`，權限 `0755`。
2. 建立 `/etc/spm`，將環境範本另存為 `server.env`／`agent.env` 並填入自己的值；設定檔權限 `0600`、擁有者 root。
3. 將需要的 service 檔放入 `/etc/systemd/system/`，執行 `systemctl daemon-reload`，再執行 `systemctl enable --now spm-server` 或 `spm-agent`。
4. 使用 `journalctl -u spm-server`／`spm-agent` 檢查狀態。一鍵安裝的 systemd 流程已在 GitHub Ubuntu amd64 runner 實測，詳見 [Release 驗證](../agent/release-validation.md)。

主程式預設只聽本機 `8080`。可使用 Caddy 的 HTTPS 反向代理（替換網域並設定 DNS）：

```caddyfile
monitor.example.com {
    reverse_proxy 127.0.0.1:8080
    header Strict-Transport-Security "max-age=31536000"
}
```

主程式必須放在網站根路徑，反向代理應保留原始 Host；API 的 Origin 檢查會比對 Host。請只對外開放 HTTPS，由同一個網站提供 API 與面板。Agent 使用外部 HTTPS URL。

SQLite 預設位於 `/var/lib/spm/spm.db`。備份時可停止服務後複製整個 `/var/lib/spm`；資料庫使用 WAL，執行中不要只複製單一 `.db` 檔。Web 資料庫設定切換後使用 `systemctl restart spm-server` 搬移；成功驗證新庫後再自行決定原庫備份的保留期限。

服務限制沒有使用 `PrivateNetwork`、`ProtectProc` 或隱藏主機掛載點，以免 Agent 採到隔離環境而非實際主機。`MemoryMax`、`CPUQuota` 可依實測自行設定，目前未宣稱固定資源占用上限。
