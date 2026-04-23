# Indexing System Design Plan

## 文件目的

這份文件不是在教你「怎麼搬 AAAgent 的程式」，而是在解釋目前 indexing system 的設計細節，讓工程師看完之後，可以在另一個專案中重新做出一套功能相近、行為一致、可維護的系統。

目標不是 code copy，而是 design transfer。

---

## 1. 系統想解決的問題

這套 indexing system 的核心問題不是「產生一個 `index.md` 檔」而已，而是要同時解決下面幾件事：

1. 能把一個資料夾的內容整理成可讀的索引與摘要。
2. 再次執行時，只重跑真的有變動的檔案，避免重複打 LLM。
3. 支援遞迴子資料夾，而且 parent folder 的描述要能反映 child folder 的內容。
4. 能在背景執行，提供進度、取消、排程與狀態查詢。
5. 當部分檔案失敗時，不要讓整個系統崩掉；下次要能重試。

所以這不是單一函式，而是一個分層系統。

---

## 2. 設計總覽

目前系統可以拆成三層：

### Layer A: Index Engine

負責真的去掃描資料夾、比對變更、摘要檔案、產生 `index.md`、寫 metadata。

核心入口：

- `llmutils.GenerateOrUpdateIndexMd(ctx, folderPath, opts)`

### Layer B: Execution Orchestrator

負責把 index engine 包成背景工作系統。

包含：

- job queue
- worker pool
- job 去重
- job 狀態追蹤
- 取消
- 進度回報
- 完成後 callback

核心模組：

- `aiagent/knowledge/indexer/manager.go`

### Layer C: Product Integration

負責把 indexing 接到產品能力上。

包含：

- DB 中 knowledge folder 的狀態更新
- auto-index scheduler
- websocket broadcast
- API wiring

這一層不是 indexing 的本質，而是產品整合。

---

## 3. 你應該先理解的核心原則

### 原則 1: Indexing 的最小單位是 folder，不是整個 workspace

系統每次操作的主體是一個 folder。這樣設計有幾個好處：

- metadata 可以跟 folder 一起放
- change detection 可以局部進行
- recursive indexing 可以自然地由 child folder 向上組裝
- background job 粒度清楚，便於 queue 與排程

### 原則 2: 增量更新靠 metadata，不靠 DB

真正決定「哪個檔案需要重跑」的依據，不在資料庫，而是在 folder 內的 `.aaagent_index_meta.json`。

這個選擇非常重要，因為：

- index engine 可以脫離產品 DB 單獨存在
- folder 可以被移動或在不同產品中重用
- 單次 indexing 的 correctness 由本地 metadata 保證，不依賴外部服務

### 原則 3: Parent folder 必須晚於 child folder 產生

系統採用 bottom-up recursion，不是 top-down。

原因是 parent folder 的描述與 `SubdirTree` 需要讀取 child folder 的 metadata。如果先做 parent，再做 child，parent 的摘要會缺資訊或過期。

所以正確順序是：

1. 先處理最深層子資料夾
2. 子資料夾寫出 metadata
3. parent 讀 child metadata
4. parent 再產生自己的 description 與 tree

這是整個系統最重要的不變條件之一。

### 原則 4: 失敗是局部狀態，不是全域中止條件

某一個檔案 extraction 失敗、摘要失敗、型別判斷失敗，不應該讓整個 folder indexing 直接失敗。

系統把失敗記在 file metadata 中，並讓其他檔案繼續。這樣：

- 可保留部分成功結果
- 下次可以只重試失敗檔案
- 整體系統對大型資料夾更穩定

---

## 4. 核心資料流

完整資料流如下：

```text
Submit folder indexing request
  -> validate path
  -> create job / or run sync
  -> recurse into subfolders first
  -> scan current folder files
  -> load old metadata
  -> compare old metadata vs current scan
  -> determine changed files
  -> summarize only changed files
  -> reuse unchanged summaries
  -> rebuild folder description
  -> rebuild subdir tree
  -> recompute folder hash + stats
  -> save .aaagent_index_meta.json
  -> write/update index.md
  -> emit result / callback / status updates
```

這裡面真正昂貴的是：

- 檔案抽取與讀取
- LLM 摘要
- vision analysis（如果啟用）

所以設計重點都圍繞在「怎麼避免不必要重跑」。

---

## 5. 核心資料模型

這些結構是工程師重建系統時最應該先定義清楚的。

### 5.1 `IndexMdOptions`

位置：

- `llm/llmutils/summarizer.go`

用途：

- 控制 index 行為
- 作為 engine 對外的主設定入口

重要欄位：

- `Recursive`: 是否遞迴
- `MaxConcurrency`: 限制並行度
- `MaxDepth`: 限制遞迴深度
- `Force`: 是否強制重建，即使檔案沒變
- `SkipHidden`: 是否略過 hidden files / dirs
- `FileTimeout`: 單檔處理上限
- `Provider`: 指定 LLM provider
- `IgnorePatterns`: ignore 規則
- `EnableImageIndexing`
- `EnableVisionAnalysis`

設計意義：

- 把「index engine 的變化點」集中在一個 options object，而不是散落在多個函式參數裡
- 讓 sync mode、background mode、scheduler mode 都能共用同一套 engine

### 5.2 `FileMetadata`

用途：

- 描述單一檔案目前的 indexing 狀態

關鍵欄位：

- `Path`
- `MD5`
- `Size`
- `ModTime`
- `Summary`
- `FileType`
- `CachedTextPath`
- `Error`
- `ProcessingType`
- `TokensUsed`

設計意義：

- 這不只是檔案資訊，也是 cache record
- 它讓系統知道哪些結果能重用、哪些要重跑

### 5.3 `FolderMetadata`

用途：

- 描述整個 folder 的 indexing snapshot

關鍵欄位：

- `Version`
- `FolderPath`
- `FolderMD5`
- `Files map[string]*FileMetadata`
- `SubdirTree`
- `LastUpdated`
- `FolderDescription`
- `BriefSummary`
- `Stats`
- `ImageHierarchy`

設計意義：

- 這是整個增量更新機制的核心
- 它不只是 cache，也是 folder state 的 single source of truth

### 5.4 `MetadataComparison`

用途：

- 表示本次掃描與舊 metadata 比較後的差異

欄位：

- `NewFiles`
- `ModifiedFiles`
- `DeletedFiles`
- `UnchangedFiles`
- `NeedsUpdate`

設計意義：

- 把「變更判斷」從「實際處理」分離
- 讓 change detection 可以單獨測試

### 5.5 `SubdirTreeNode` / `SubdirInfo`

用途：

- 表示 child folder 的摘要樹

裡面放的不是完整 metadata，而是 parent 需要用到的 child 摘要資訊：

- brief description
- detailed description
- last indexed time
- file count
- subdir count
- has index
- children

設計意義：

- parent 不需要重新掃 child 全內容
- parent 只依賴 child 已產生好的 metadata，降低耦合與成本

### 5.6 `MetadataStats`

用途：

- 預先快取 folder 統計資訊

目前包含：

- `TotalFiles`
- `IndexedFiles`
- `NonIndexedFiles`
- `ErrorFiles`
- `BinaryFiles`

設計意義：

- 避免每次查 stats 都重掃整個 folder metadata
- 讓 UI / API 可以快速展示現況

---

## 6. 增量更新設計

這是這套系統最值得保留的部分。

### 6.1 為什麼用 MD5

每次掃描 folder 時，系統會對每個檔案計算 MD5，並存進 metadata。

原因：

- `modtime` 不可靠，可能變了但內容沒變
- `size` 不可靠，內容可能改了但大小相同
- MD5 對這種快取判斷已經夠用，而且成本可接受

### 6.2 `compareMetadata` 的行為

`compareMetadata(...)` 做幾件事情：

1. 找出新檔案
2. 找出 MD5 改變的檔案
3. 找出已刪除檔案
4. 找出未變動檔案
5. 比較子資料夾集合是否改變
6. 決定 `NeedsUpdate`

另外有兩個非常重要的設計細節：

#### 細節 A: 之前失敗過的檔案，即使內容沒變也要重試

如果某個檔案的舊 metadata 裡面有 `Error`，而本次內容沒變，系統仍然把它視為 `ModifiedFiles`。

原因是：

- 上次失敗可能是暫時性錯誤
- 如果只看 MD5，這類檔案會永遠卡在失敗狀態

#### 細節 B: 未變動檔案會重用舊摘要與 cache path

對於 `UnchangedFiles`，系統會把舊 metadata 中的：

- `Summary`
- `CachedTextPath`
- `FileType`

複製到新的 file metadata。

這代表：

- 本次不用重跑摘要
- 仍能組裝新的整體 folder description

### 6.3 Skip 也不是完全不做事

當 `NeedsUpdate == false` 時，系統不是直接 return。

它仍然會：

- rebuild `SubdirTree`
- 更新 `LastUpdated`
- 必要時把新的 `SubdirTree` 寫回 metadata

原因是：

- child folder 可能剛剛被重建
- parent 自身檔案雖沒變，但 child 的 brief summary 可能變了

這是另一個非常重要的不變條件。

---

## 7. 遞迴與並行設計

### 7.1 為什麼是 bottom-up recursion

`GenerateOrUpdateIndexMd(...)` 的做法是：

1. 如果 `Recursive`，先呼叫 `generateIndexRecursive(...)`
2. 等 child 全部處理完
3. 再呼叫 `generateIndexForSingleFolder(...)` 處理當前 folder

這確保 parent 產生描述時可以讀到 child metadata。

### 7.2 為什麼要 shared semaphore

系統使用 shared folder semaphore 控制整體並行度，而不是讓每一層 recursion 自己無限制開 goroutine。

要解決的問題：

- 深層資料夾樹容易 goroutine 爆量
- 單靠每層 local concurrency 無法限制總量

### 7.3 為什麼 semaphore 只包 leaf folder processing，不包整段 recursion

這是程式裡特別小心處理的點。

如果 parent 在遞迴進 child 時就先佔住 semaphore，深度夠大、`MaxConcurrency` 又不高時，會出現 self-deadlock：

- parent 持有 slot
- child 需要 slot 才能完成
- 但 slot 都被祖先佔住

所以現在的設計是：

- recursion 本身不持有 semaphore
- 只有真正執行 `generateIndexForSingleFolder(...)` 時才 acquire

這是重建時一定要保留的設計。

---

## 8. 單一 folder 的處理流程

`generateIndexForSingleFolder(...)` 大致是以下步驟：

1. 掃描 folder 檔案並計算 MD5
2. 列出 immediate subdirectories
3. 載入舊 metadata
4. 比對新舊差異
5. 若無需更新，走 skip 流程
6. 若需更新，找出要處理的檔案
7. 並行處理 changed files
8. 重建 folder description / brief summary
9. 重建 `SubdirTree`
10. 計算 `FolderMD5`
11. 計算 `Stats`
12. 寫回 metadata
13. 回傳 `IndexingResult`

這裡面最值得注意的是第 7 步。

### 8.1 changed files 才進 LLM pipeline

系統不會重跑所有檔案，只處理：

- new
- modified
- previous failed
- `Force == true` 時的所有檔案

### 8.2 unchanged files 仍然參與 folder description 組裝

雖然 unchanged file 不重新摘要，但它們的舊 summary 仍保存在 metadata 裡，後續組 folder-level description 時還是能用。

所以 engine 的真正目標不是「處理所有檔案」，而是「得到完整且最新的 folder state」。

---

## 9. Metadata file 為什麼要設計成 `.aaagent_index_meta.json`

目前 metadata 放在 folder 內，與 `index.md` 同層。

這個設計有幾個實際優點：

1. folder 可攜
2. engine 可離線運作
3. 不依賴外部 DB 才能做 change detection
4. debugging 容易，工程師可直接看 JSON
5. recursive parent/child 組裝容易

建議在新系統裡保留這個思路，即使檔名不一定要完全相同。

如果要改檔名，也建議保留這些屬性：

- local to folder
- human-inspectable
- stable schema version
- 足夠支撐增量更新

---

## 10. 背景工作系統怎麼包 engine

如果產品不只要 sync indexing，而是要背景 job，AAAgent 的做法值得參考。

### 10.1 `IndexManager` 的責任

`IndexManager` 不做 indexing 演算法本身，它只做 orchestration：

- queue job
- deduplicate path
- 管理 running / completed / failed counters
- 啟 worker
- 把 `ProgressReporter` 接到 job state
- 完成後呼叫 callback

這是好的設計，因為它讓 engine 保持純粹。

### 10.2 為什麼用 `pathInUse`

`SubmitJob(...)` 會先檢查 `pathInUse[cleanPath]`。

目的：

- 防止同一個 folder 同時被多次索引
- 避免 metadata / index.md 被競爭性寫入
- 避免重複浪費 LLM 成本

對這類系統來說，folder-level mutual exclusion 幾乎是必要的。

### 10.3 callback 為什麼做成 interface

`IndexManager` 透過：

- `FolderIDResolver`
- `IndexCompletionCallback`

把自己和產品 DB 解耦。

這個設計很好，因為 queue system 不應該知道產品資料表 schema。

在新系統裡建議保留這個做法。

---

## 11. Progress 設計

engine 透過 `ProgressReporter` 介面回報進度：

- `Report(info ProgressInfo)`
- `ReportError(err error)`
- `Complete()`

這個設計的重點不是 UI，而是 decoupling：

- sync CLI 可以印 log
- background job 可以更新 in-memory status
- websocket 可以廣播
- future 也可以接 tracing / metrics

`ProgressInfo` 目前描述：

- stage
- path
- current
- total
- percentage
- message

這樣的設計比只回傳百分比更有用，因為 indexing 是多階段流程，不只是單純 for-loop。

---

## 12. 失敗與恢復策略

### 12.1 file-level failure

在掃描、型別判斷、抽取、摘要、vision analysis 的任何步驟失敗，都盡量記錄到對應 file metadata，而不是讓整個 folder fail fast。

這樣的好處：

- 大資料夾不會因為一個壞檔案全部失敗
- 可以保留已成功結果
- 下次可重試失敗檔案

### 12.2 job-level failure

如果是更高層級錯誤，例如：

- folder path 不存在
- context cancelled
- folder metadata 讀寫出現致命錯誤

這種才應該讓 job fail 或 cancel。

### 12.3 cache cleanup

`compareMetadata(...)` 在發現檔案被修改，或舊紀錄有 error 時，會刪掉舊的 `CachedTextPath`。

這個細節很重要，因為：

- 否則可能會誤用舊 cache
- partial/corrupt cache 會殘留

---

## 13. Parent description 是怎麼建立的

雖然細部 prompt 與內容生成邏輯在 `summarizer.go` 裡比較多，但設計上可以把它理解成：

1. 收集當前 folder 的 file summaries
2. 收集 child folders 的 brief/detailed summaries
3. 交給 LLM 生成：
   - `FolderDescription`
   - `BriefSummary`
4. 若 LLM 不可用，退化成 fallback description

這裡的設計重點是：

- folder description 是 derived data，不是 primary data
- primary data 是 file metadata + child summaries

所以就算 prompt 未來調整，只要 metadata layer 穩定，整個系統仍容易演進。

---

## 14. 影像索引是可選擴充，不應污染核心抽象

`IndexMdOptions` 中有 image/vision 相關欄位，`FileMetadata` 中也有 image fields。

這在目前專案裡合理，但如果你在另一個專案重建，建議把影像能力視為 extension，而不是 indexing core。

比較好的抽象方式是：

- core engine 處理 generic file indexing
- document extractor / image analyzer 當 plugin
- metadata schema 容納 optional extension fields

這樣系統更容易長期維護。

---

## 15. 工程師重建這套系統時，建議的模組切法

如果要在新專案裡重新做，我建議至少拆成下面幾個 package：

### `indexengine`

責任：

- folder scan
- metadata compare
- changed file selection
- per-file summarize pipeline
- folder description generation
- metadata persistence

### `indexmodel`

責任：

- `FileMetadata`
- `FolderMetadata`
- `MetadataComparison`
- `ProgressInfo`
- `IndexResult`

### `indexruntime`

責任：

- queue
- worker pool
- cancellation
- dedupe
- callback hooks

### `indexintegration`

責任：

- DB update
- scheduler
- websocket / event bus
- API

這樣可以避免把產品需求混進核心演算法。

---

## 16. 建議的實作順序

這是我認為最穩的重建順序。

### Phase 1: 先做純同步 engine

先完成：

- scan folder
- load/save metadata
- compare metadata
- process changed files only
- generate folder description
- write `index.md`

驗證標準：

- 第一次跑會建立 metadata
- 第二次沒變時會 skip
- 改一個檔案時只重跑那個檔案
- 刪一個檔案時 metadata 會更新

### Phase 2: 補 recursive bottom-up

先確保：

- child 先完成
- parent 能讀 child metadata
- skip 模式下 parent 仍會更新 `SubdirTree`

### Phase 3: 補 progress/cancellation

先接：

- context cancel
- staged progress reporting

### Phase 4: 再包 background job manager

這時才做：

- queue
- path dedupe
- job status
- callback

### Phase 5: 最後才做 scheduler / websocket / failure UI

這些是產品能力，不是 index correctness 的核心。

---

## 17. 這套設計中最重要的不變條件

如果工程師只記五件事，我會要他記這五件：

1. Parent folder 必須在 child folder 之後產生。
2. 增量更新的真實來源是 folder-local metadata，不是 DB。
3. Unchanged files 必須重用舊 summary，否則增量更新沒有意義。
4. Previous failed files 即使 MD5 沒變，也必須重試。
5. 同一路徑不能同時有兩個 indexing job，否則 metadata 會互相覆蓋。

---

## 18. 對另一個專案的實作建議

如果另一個專案要「做出類似系統」，我建議：

### 建議 1

先重做 engine，不要先重做 scheduler。

### 建議 2

metadata schema 先求穩定，不要一開始就想做很漂亮的最終版本。

### 建議 3

把 compare logic 做成可單元測試的純函式，這是整套系統 correctness 的核心之一。

### 建議 4

把 file extraction / summarization / image analysis 做成可替換模組，不要直接耦合在一個巨型流程中。

### 建議 5

把 folder-level lock/dedupe 當成必要條件，不是優化。

---

## 19. 目前程式碼中最值得直接參考的部分

如果工程師要讀現有程式碼，我建議按這個順序讀：

1. `llm/llmutils/summarizer.go`
   - `GenerateOrUpdateIndexMd`
   - `generateIndexRecursive`
   - `generateIndexForSingleFolder`
   - `compareMetadata`
   - `scanFolderFiles`
   - `loadMetadata`
   - `saveMetadata`

2. `aiagent/knowledge/indexer/manager.go`
   - 看 engine 如何被包成 job system

3. `aiagent/knowledge/indexer/job.go`
   - 看 job state model

4. `aiagent/knowledge/indexer/ignore.go`
   - 看 ignore pattern 設計

5. `aiagent/knowledge/scheduler/autoindexer.go`
   - 最後再看排程如何接上

---

## 20. 結論

AAAgent 的 indexing system 本質上是一個「以 folder-local metadata 為核心、採用 bottom-up recursion、支援增量更新與背景執行」的內容索引系統。

如果要在另一個專案中重建類似能力，最應該複製的不是某個 API 形狀，而是這四個設計：

1. local metadata 驅動的增量更新
2. child-first 的遞迴順序
3. file-level failure isolation
4. engine 與 orchestration 分層

只要這四個設計被保留，外層是 CLI、API、queue worker、scheduler，甚至 metadata 檔名不同，都還能做出行為一致的系統。
