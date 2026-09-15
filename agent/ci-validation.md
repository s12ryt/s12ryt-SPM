# GitHub 真實 Linux CI

## 執行環境與驗收

- 儲存庫：https://github.com/s12ryt/s12ryt-SPM（私有）。
- Workflow：`.github/workflows/linux-ci.yml`，由 main 推送、Pull Request 或手動觸發。
- GitHub-hosted Ubuntu 24.04 amd64；Go 1.26.3、Node 24、PostgreSQL 17 service。
- 每次 job 最多 20 分鐘；測試資料庫與 binary smoke 資料皆為隔離測試資料。
- 前端 `npm ci`、12 項行為測試、型別檢查與嵌入資源建置。
- Go 格式、vet、module verify 與 `go test -race -json ./... -count=1 -timeout=5m`。
- race 沒有排除任何案例；解析 JSON 確認 PostgreSQL、Upload、RunUploadsAndStops、WebhookPayloadAndFailure 都為 PASS，任何測試案例 SKIP 讓 CI 失敗（無單元測試的套件層級 skip 除外）。
- Linux amd64 真實執行檔 smoke：私人權限、登入、真實 OS 採集、兩筆樣本 CPU 速率、歷史、Token 隱藏及設定。
- Linux ARM64 僅交叉編譯，不等同 ARM64 真機驗證。
- 測試 JSON 與 Linux 產物保留 14 天，可從 Actions run 下載。

## 實際結果

第一輪：https://github.com/s12ryt/s12ryt-SPM/actions/runs/34949650563

提交：`d1f68297650ebc5ba37fe68f3f18b76f4e288c70`。Go 全套 race 與 PostgreSQL PASS，但結果檢查器誤把 `spm/cmd/agent`（沒有單元測試）套件層級 skip 當成測試略過，workflow 因此失敗，binary smoke 尚未執行。

修正前以 Python 5 項回歸測試重現 2 項錯誤：無測試套件被拒絕、檢查器未單獨拒絕 fail 事件。修正只允許沒有 Test 欄位的套件 skip，仍拒絕實際測試 skip、fail 或缺少必要 PASS。Go 指令的失敗退出仍由 pipefail 保護。

本機 actionlint 通過；此次 workflow 組態以遠端執行驗收，沒有將語法檢查宣稱為產品 TDD RED。

第二輪：https://github.com/s12ryt/s12ryt-SPM/actions/runs/34950376689

提交：`a17530c393fbb46f6b01fe5d4ee5dba2a0b6437f`，最終狀態 **success**。

- 前端 12 項行為測試、TypeScript 型別檢查與正式建置通過。
- Go 全套 race 通過，沒有排除任何測試；真實 PostgreSQL 17 的雙向搬移、歷史分桶、節點狀態與通知佇列測試通過。
- 原本 WSL 受阻的三項 HTTP 測試全部通過；實際測試案例沒有 skip。Agent 入口套件沒有單元測試，另由真實 binary smoke 驗證。
- Python 結果檢查器 5 項測試通過；下載此輪 `linux-test-results` 後在本機重新驗證 JSON 也通過。
- Ubuntu 上建置並執行主程式與 Agent，完成真實採集、登入與權限、CPU 衍生速率、歷史和設定 smoke。
- ARM64 交叉建置、artifact 上傳及容器清理均成功。
- `linux-test-results` 與 `linux-binaries` 可由此 run 頁下載，保留 14 天。

本次共 2 輪 CI。後續僅紀錄更新的提交使用 `[skip ci]`，不變更已驗證的程式、測試與 workflow。ARM64 真機、systemd、外部通知及長時間壓力測試仍不在此次實測範圍。
