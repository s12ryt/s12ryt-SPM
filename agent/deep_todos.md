# 任務歷史

## 2026-09-16：具體程式缺陷檢查

- [x] 重讀驗收與核心程式、前端、兩種安裝器，建立 Go／Vue／Python 基線。
- [x] TDD 修正 SQLite 目錄 URL 與檔案 DSN 的特殊字元解析。
- [x] TDD 修正刪除主機後舊輪詢成功／失敗覆寫新畫面。
- [x] TDD 修正兩個安裝器中途寫檔失敗未回復，補首次失敗重試回歸。
- [x] Go 全套／vet／模組檢查、Linux store race、Vue 14 項／型別／建置及 Python 24 項通過。
- [x] Windows 真實 Server／Agent smoke 通過；主代理完成修補後複查。
- [ ] 提交推送並驗證本輪完整 Linux race、PostgreSQL、兩種免 root 安裝 CI。
- [ ] 保存 CI 證據並確認工作區狀態；詳見 `audit-2026-09-16.md`。

## 2026-09-15：免 root VPS 一鍵安裝

- [x] 使用者確認自動選擇 systemd --user／Supervisor，寫入需求驗收。
- [x] TDD：使用者 HOME、角色隔離、固定 Release 與校驗、秘密讀取、更新保留及回復。
- [x] 回歸修正 Supervisor 繼承安裝／控制鎖，8 項免 root 測試通過。
- [x] 與原安裝器及 CI 檢查器合計 20 項 Python 測試通過。
- [x] 建立兩個真實 Ubuntu 非 root 安裝驗收 job，下載正式 v0.1.0 執行檔。
- [x] 推送並確認兩種管理方式的真實安裝／上報／啟停／崩潰重啟／更新驗證。
- [x] 修正 CI 一般帳號無法讀取 checkout 的隔離 fixture 路徑，第二輪三個 job 全成功。
- [x] 更新遠端證據與使用方式；詳見 `user-install-validation.md`。

## 2026-09-15：公開儲存庫、一鍵安裝與 Release

- [x] 確認預設只裝 Server、`agent` 參數只裝 Agent；版本標籤發布。
- [x] GitHub 儲存庫切換為 PUBLIC。
- [x] TDD 安裝器：7 組測試，角色／平台／版本／校驗／設定保留／秘密／失敗回復。
- [x] 重用完整 CI 建置 Release，加入真實 Release systemd 安裝驗收。
- [x] 推送並執行首版 v0.1.0，Release workflow 首次成功。
- [x] 從正式 Release 真實安裝 systemd、上報與更新保留驗收成功。
- [x] 更新 `release-validation.md`，保存資產、提交與遠端證據。

## 2026-09-15：GitHub 真實 Linux CI

- [x] 確認帳號 s12ryt，建立私有 s12ryt-SPM 儲存庫。
- [x] 建立 Ubuntu 24.04、PostgreSQL 17、完整 race 與真實 Agent smoke workflow。
- [x] actionlint 驗證 workflow；排除本機產物、瀏覽器紀錄與截圖。
- [x] 推送並確認遠端執行結果，修正檢查器誤判後第二輪全部成功。
- [x] 更新 PostgreSQL／完整 race 驗證限制與 CI 證據，詳見 `ci-validation.md`。

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

- [x] GitHub Ubuntu runner 上 PostgreSQL 17 實例的 `TestPostgresIntegration` 通過。
- [x] GitHub Ubuntu runner 上含 HTTP 的全套 race 通過，沒有排除案例。
- [x] Ubuntu amd64 真實 Release systemd 安裝／更新與 Agent 上報，見 `release-validation.md`。
- [ ] ARM64 真機與外部 Telegram／Webhook 投遞驗證；目前完成交叉編譯與模擬測試。

本輪交付為 7 個工作週期：原 6 個架構／實作／整合週期，加 1 個回歸與交付收尾週期。限制詳見 `validation.md`。
