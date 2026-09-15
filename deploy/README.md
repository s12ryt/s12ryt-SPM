# Linux 部署範例

建議使用 [README 一鍵安裝](../README.md#vps-一鍵安裝)，從 Releases 取得已建置程式並建立 systemd 服務。下列為手動安裝替代步驟；一鍵安裝預設 Server 監聽 `0.0.0.0:8080`，手動範本則為 `127.0.0.1:8080`。

以下操作由管理系統的人員在目標 VPS 執行。採一般 `spm` 帳號執行主程式與 Agent；收集一般主機資訊不需要 root。

1. 建立專用 `spm` 使用者，將對應 CPU 架構的執行檔安裝成 `/opt/spm/spm-server` 或 `/opt/spm/spm-agent`，權限 `0755`。
2. 建立 `/etc/spm`，將環境範本另存為 `server.env`／`agent.env` 並填入自己的值；設定檔權限 `0600`、擁有者 root。
3. 將需要的 service 檔放入 `/etc/systemd/system/`，執行 `systemctl daemon-reload`，再執行 `systemctl enable --now spm-server` 或 `spm-agent`。
4. 使用 `journalctl -u spm-server`／`spm-agent` 檢查狀態。範本只供參考，本次未在 systemd 實際安裝。

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
