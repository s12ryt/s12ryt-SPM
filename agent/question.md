# 需求與 TDD 驗收標準

## 免 root VPS 一鍵安裝（2026-09-15）

- 使用者要求提供免 root 一鍵安裝，允許引用外部程序管理工具；確認選擇「自動選擇」：優先 systemd --user，否則使用獨立 Python venv 的 Supervisor。
- 新增 `install-user.sh`；不加參數只安裝 Server，`agent` 只安裝 Agent。SPM 執行檔仍只能來自正式 Release 且必須驗證 SHA-256。
- 所有執行檔、設定、資料與服務檔只寫入目前使用者 HOME；不執行 sudo、useradd、chown，不變更系統服務。
- 首次自動選擇管理方式，更新保留既有管理方式、設定及資料；下載／校驗失敗不替換，啟動失敗嘗試還原舊執行檔與服務設定。
- 提供 `~/.local/bin/spm-user server|agent start|stop|restart|status|logs`，限時控制、Supervisor socket 私有且日誌輪替。
- Supervisor 備援需要 Python 3／venv；依賴不足明確提示，不嘗試提升權限。開機自動啟動／登出存活受主機 linger 與登入政策限制，不能宣稱一般帳號可保證。
- 以 TDD 驗證兩種管理方式、角色隔離、特殊字元、設定保留及錯誤復原；GitHub 真實 Linux runner 使用非 root 帳號從正式 Release 安裝複驗。

## 公開儲存庫與 Release 安裝（2026-09-15）

- 使用者要求 GitHub 儲存庫改為公開。
- 根目錄 `install.sh` 不加參數預設只安裝 Server；第一個參數 `agent` 才只安裝 Agent，不連帶安裝另一端。
- 使用 Bash process substitution：`bash <(curl -fsSL https://raw.githubusercontent.com/s12ryt/s12ryt-SPM/main/install.sh)`；Agent 在指令最後加 `agent`。
- 推送版本標籤自動測試、建置與發布 Releases，本次發布 `v0.1.0`。
- 安裝脚本只能下載正式 Releases 已建置的執行檔，不下載原始碼編譯、不使用 Actions artifact 作為安裝來源。
- Linux amd64／arm64 VPS，沿用 systemd 部署；初次要求必要設定，更新保留既有設定和資料。
- 下載固定版本資產並驗證 SHA-256，下載／校驗失敗不得替換既有執行檔；預設最新版，提供 `--version vX.Y.Z` 指定版本。
- 測試角色隔離、平台拒絕、必要設定、秘密不外洩、下載／校驗錯誤、設定保留與服務失敗回報。CI 使用真實 Release 產物複驗安裝。

## 使用者已確認（2026-09-15）

- 以 Go 分別編寫主程式與 Agent，產出獨立執行檔。
- Agent 維持輕量，只採集並上傳系統資訊；計算速率、資料保存、狀態判斷與告警由主程式執行。
- 前端採 Vue + TypeScript，建置後嵌入 Go 主程式。
- 預設只需 SQLite；可透過環境變數或 Web 設定 PostgreSQL。
- 資料庫環境變數只接受 URL 形式，不提供分散的 host / port / user / password 變數。
- 每 3 秒探測並上報一次。
- 首版包含多機即時狀態、歷史圖與告警。
- Agent 支援 Linux 與 Windows。
- 面板可公開或私人，由管理員決定。

## 第二輪確認

- 網頁內告警固定提供；Webhook 與 Telegram 為可選通知管道，可分別停用。
- Web 設定新資料庫 URL 後，重啟生效並自動搬移主機、歷史、設定與告警；成功前保留原資料庫。
- DATABASE_URL 環境變數優先於 Web 儲存設定；設定環境變數時不套用待處理搬移。
- CPU／RAM 預設超過 90% 持續 30 秒告警，磁碟超過 90% 告警，15 秒未上報判定離線。
- 管理員可調整全域告警規則，個別主機可覆寫；觸發與恢復通知避免重複。
- Agent 上報間隔也必須可調整，預設 3 秒；全域設定與單機覆寫皆支援。

## 實作契約

- 主程式回傳下一次上報間隔，Agent 不接收遠端命令或執行額外探測任務。
- 避免長間隔造成誤報，實際離線期限至少為上報間隔的三倍加 3 秒。
- 歷史預設保留 7 天，查詢以時間分桶限制輸出筆數。
- 資料搬移使用目標交易與完成標記，可在設定檔切換失敗後重試；有不相關既有資料的目標會被拒絕。
- 管理員透過環境變數設定登入帳密；各台 Agent 使用獨立 Token，資料庫只保存 Token 雜湊。
- 預設私人面板；公開模式唯讀，不公開通知憑證、Agent Token 或資料庫連線資訊。
- 介面採繁體中文、深色監控面板，提供搜尋、主機詳情、趨勢、告警、全域及單機設定。

## 初步驗收

- 未經授權的 Agent 無法上報；私人模式的訪客無法取得監控資料。
- 公開模式訪客可讀取監控資料，但不能修改設定。
- Agent 不負責持久化歷史、判斷離線或寄送告警。
- 系統處理計數器重設、第一次採樣、離線、異常上報與資料庫錯誤。
- 新增行為依 RED → GREEN → REFACTOR 建立可執行測試證據。
