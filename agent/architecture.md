# 架構與測試追蹤

## 邊界

`cmd/agent` → `internal/collector` → 作業系統唯讀資訊 → HTTPS JSON → `internal/server` → `internal/store` → SQLite / PostgreSQL。

`web/src` → 管理員 Session 或公開唯讀 API → 主程式。Vue 產物由 `web/embed.go` 打包。

Agent 傳原始 CPU 累積時間、記憶體／磁碟容量、每網卡累積流量與開機時間。主程式以採樣時間差計算 CPU 使用率及網路速率。第一次採樣、重開機與計數器回退不產生不合理尖峰。

## 測試週期

1. 資料模型：輸入驗證、衍生速率、全域／單機規則與告警狀態。
2. 資料庫：URL、CRUD、歷史分桶、保留期限、交易搬移與重試。
3. HTTP：登入／登出、CSRF、公開／私人、Agent Token、上報、設定、歷史與告警。
4. 通知與採集：Webhook／Telegram、錯誤與取消、系統採集、間隔下發、HTTP timeout。
5. 前端：登入、真實資料顯示、空態、管理設定、錯誤、主機覆寫與趨勢。
6. 整合：完整測試、race（環境允許時）、vet、Vue 型別檢查與 Linux／Windows 建置。
7. 回歸與交付：慢請求、排序、覆寫、同名告警、磁碟歷史、瀏覽器複驗與部署文件；結果詳見 `validation.md`。

## 決策

- SQLite 使用 WAL 與有限連線；共用 database/sql 契約支援 PostgreSQL。
- 監控資料每 3 秒輪詢，HTTP ETag 並非必要；單次請求完成後才排程下一次，避免請求重疊。
- 每次樣本寫入後評估資源告警；主程式定時評估離線。告警轉換及通知待送狀態持久化。
- 通知工作單獨執行，設定短 timeout 與有限重試，避免延遲 Agent 上報。
- 重啟搬移至空目標，以單一目標交易寫入所有資料和搬移 ID；設定原子替換後才切換。
- 不使用子代理。
