# 免 root 一鍵安裝驗證

日期：2026-09-15。使用者選擇自動判定：優先 `systemd --user`，不可用時使用私人 Python venv 的 Supervisor。預設只安裝 Server；`agent` 參數只安裝 Agent。SPM 執行檔只取自正式 Releases，校驗 SHA-256，不在 VPS 編譯。

## 遠端實測

- [成功的 Linux CI](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34957308410)
- 驗證提交：`3670974e18445f97ceb6358aad5c60a8abe52fdb`。
- Ubuntu 24.04.5 amd64，兩個獨立 runner，各建立 UID 1002 的 `spm-rootless` 一般帳號。
- [systemd 使用者服務 job](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34957308410/job/104342347924)：成功。
- [Supervisor job](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34957308410/job/104342348198)：成功，Python 3.12 venv 實際下載並安裝 Supervisor 4.3.0。
- 下載既有正式 [v0.1.0](https://github.com/s12ryt/s12ryt-SPM/releases/tag/v0.1.0)；本次未改動 Go／Vue 程式或覆寫已發布資產。

準備階段僅使用 runner 管理權限建立測試帳號、複製兩個測試檔，以及為 systemd 分支建立可用的使用者服務環境。安裝器、程序管理、Server 與 Agent 全程由一般帳號執行，測試透過 `/proc/PID` 確認服務屬於相同 UID。

兩個 job 均實際驗證：

1. 不帶角色參數只安裝 Server，首次登入可用，私人 API 不公開。
2. 密碼含 `$HOME`、雙引號、反斜線、空白與 `%`，登入驗證原值正確。
3. 新增主機後單獨安裝 Agent，Server 執行檔及設定保持不變。
4. 收到至少兩筆真實系統樣本，CPU 衍生率非空且主機在線。
5. 對測試帳號的 Agent 發送 SIGKILL，程序管理器會以新 PID 自動重啟。
6. `status`、`logs`、`stop`、`start` 可用；停止 Server 後 HTTP 確實停止接受連線。
7. 重跑安裝保留原密碼、設定與資料庫中的主機，兩個角色仍正常運作。
8. 設定檔權限 `0600`；Supervisor UNIX socket 權限 `0600`，無 TCP 控制介面。
9. finally 停止兩個角色，systemd 停用測試 unit；workflow 清理測試使用者程序與 linger。

兩種管理器的日誌均包含：

```text
PASS: uid=1002, <manager>, Release-only install, role isolation, literal secrets,
real samples, crash restart, stop/start, update preservation
```

## TDD 與回歸

- 初始 stub 造成 5 個有效行為 RED：預設安裝、Supervisor 備援、更新保留、失敗回復及秘密載入尚未實作。測試 fixture 缺少空操作紀錄的錯誤另行修正，不計為產品 RED。
- 實作後目標測試通過，再新增描述符檢查，確認 Supervisor 啟動會繼承安裝與控制鎖，形成有效 RED。
- 啟動 Supervisor 前關閉描述符 8／9，背景程序不再持有這兩把鎖；8 項免 root 測試 GREEN。
- 原 root 安裝器 7 項、免 root 安裝器 8 項、Go 結果檢查器 5 項，共 20 項 Python 測試於本機 WSL 與遠端 Ubuntu 通過。
- 本地測試將 HOME、下載與系統服務邊界隔離在暫存目錄，未安裝本機正式服務。
- Bash 語法、ShellCheck、actionlint 通過。
- 同次遠端 CI 的 Vue 12 項測試、型別與建置、Go 完整 race、真實 PostgreSQL 17 雙向搬移、Go vet／模組驗證、Linux binary smoke 及 ARM64 交叉建置均成功。

## CI 環境修正

[第一次執行](https://github.com/s12ryt/s12ryt-SPM/actions/runs/34957094027)的一般帳號無權穿越 runner checkout 路徑，Python 尚未執行安裝驗收便得到 Permission denied；這是測試環境問題，不是產品安裝失敗。

改為僅將 `install-user.sh` 與 `user_install_smoke.py` 放進測試帳號自己的家目錄，再以該帳號執行；沒有放寬 runner 家目錄權限。第二次執行三個 job 全部成功。

本次共 3 輪工作週期：免 root 行為 TDD、背景鎖回歸、遠端實機與交付驗證；遠端 workflow 執行 2 次。

## 使用限制

- Linux amd64 兩種程序管理器均已實機驗證；ARM64 為資產交叉編譯及安裝器架構選擇測試，尚未 ARM64 真機驗證。
- Supervisor 分支需要可用的 Python 3／venv 與 PyPI 連線；依賴固定 4.3.0，安裝於自己家目錄。
- systemd 分支的持續登出／開機執行需要業者允許 linger；CI 有明確準備此條件，不代表所有 VPS 預設具備。
- Supervisor 自動重啟子程序，但本腳本不設定 crontab 或其他開機機制；重開機後需 `spm-user ... start`。業者強制清理帳號程序的政策無法由腳本繞過。
- 安裝與更新需要家目錄可寫，Supervisor 的 UNIX socket 路徑有長度上限；一般帳號應使用業者允許的非特權連接埠。
- 更新失敗嘗試還原舊執行檔、啟動器及服務設定，不回復資料庫；更新前仍須依需求備份。
- 後續收尾提交只更新文件；以上提交的安裝器與 workflow 為本次驗證版本。
