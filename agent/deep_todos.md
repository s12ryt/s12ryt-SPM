# 任務歷史

## 2026-09-15：GitHub 真實 Linux CI

- [x] 確認帳號 s12ryt，建立私有 s12ryt-SPM 儲存庫。
- [x] 建立 Ubuntu 24.04、PostgreSQL 17、完整 race 與真實 Agent smoke workflow。
- [x] actionlint 驗證 workflow；排除本機產物、瀏覽器紀錄與截圖。
- [ ] 推送並確認遠端執行結果，失敗時修正後重跑。
- [ ] 更新 PostgreSQL／完整 race 驗證限制與 CI 證據。

## 2026-09-15：建立 Go VPS 即時監控系統

- [x] 確認工作區為空，讀取全域規範與 TDD 技能。
- [x] 確認 Vue + TypeScript、SQLite／PostgreSQL URL、3 秒採樣、Linux／Windows、公開切換。
- [x] 確認 Web 固定紀錄、選用 Webhook／Telegram；重啟自動搬移、環境變數優先。
- [x] 設計並測試主程式、Agent、資料庫與驗證契約。
- [x] 實作 Vue 面板、歷史圖、管理設定與告警。
- [x] 真實 Windows／Linux 採集、上報與取消；Vue 7 項測試。
- [x] 回歸修正列表排序、單機覆寫預設值、同名告警合併、磁碟歷史及慢請求阻塞。
- [x] Go 全套測試、go vet、Vue 型別檢查與正式建置通過。
- [x] Windows amd64、Linux amd64／arm64 主程式與 Agent 建置完成。
- [x] Windows／Linux 執行檔整合通過；瀏覽器桌面與手機版、管理流程驗證完成。
- [x] model／store race 通過；HTTP race 的 WSL 連線限制已記錄，不宣稱全套 race 通過。
- [x] PostgreSQL 隔離 schema 整合測試入口已建立；無實例時明確略過，保留待驗證限制。
- [x] README、systemd／環境範本、驗證紀錄與檔案依賴圖完成。
- [x] 臨時服務到期自動清理，確認 Windows／WSL 無本次測試進程殘留。

## 2026-09-15：依使用者要求深入稽核

- [x] 重讀驗收與所有核心實作、建立既有測試基線。
- [x] 以失敗測試修正四種告警連續性問題。
- [x] 以失敗測試修正前端權限逾期、延遲回應、逾時取消、刪除及導覽競態。
- [x] 修正 SQLite URL 邊界與損壞 Web 設定阻擋 DATABASE_URL 的問題。
- [x] 確認搬移中途回滾、私人／逾期權限與通知重新導向。
- [x] 修正 HTTPS Cookie、監聽失敗 exit code、大回應及 bcrypt 共用鎖阻塞。
- [x] Go 全套／vet、Vue 12 項測試／型別／建置及選定範圍 race 通過。
- [x] 重新建置六個執行檔，Windows／Linux 真實上報 smoke 通過。
- [x] 瀏覽器複驗 Session 失效、選中主機被刪除、五種圖表及手機寬度。
- [x] 建立 `audit-2026-09-15.md`，區分真實 RED、首次即通過的檢查及環境限制。
- [x] 最後一個 TTL 瀏覽器測試程序已清理，Windows／WSL 無本次服務殘留。

本次深入稽核另計 5 輪工作週期，詳見稽核紀錄；保留首次交付歷史。

## 驗證限制

- [ ] 於可連線的 PostgreSQL 實例執行 `TestPostgresIntegration`。
- [ ] 在本機 loopback 正常的環境完成含 HTTP 的全套 race。
- [ ] ARM64 真機、systemd 與外部 Telegram／Webhook 投遞驗證；本次完成交叉編譯、範本與模擬測試。

本輪交付為 7 個工作週期：原 6 個架構／實作／整合週期，加 1 個回歸與交付收尾週期。限制詳見 `validation.md`。
