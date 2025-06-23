# tw (tmux worker) manager

tmuxベースのワーカー管理CLIツール。Issue番号や機能名を指定して、以下を自動的に作成・管理します：

- tmuxセッションとペイン分割
- git worktreeの作成
- init commandの実行

## 機能

- **init/destroy**: tmuxセッションの初期化・削除
- **add**: 新しいワーカーを作成（設定されたcommandを起動）
- **list**: 全ワーカーの一覧表示
- **remove**: ワーカーの削除
- **status**: 特定ワーカーの詳細状態表示
- **attach/detach**: tmuxセッションへの接続・切断
- **check/repair**: worktreeとpaneの整合性チェック・修復
- **config**: コマンド設定の管理

## tmuxセッション名の命名規則

tmuxセッション名は `<project>` の形式で作成されます。
`<project>` は現在のディレクトリ名が使用されます。

例：プロジェクトディレクトリが `my-awesome-project` の場合、セッション名は `my-awesome-project` になります。

## プロジェクトディレクトリ制約

セキュリティと整合性のため、以下の制約があります：

- **初期化ディレクトリの記録**: `tw init`実行時に現在のディレクトリパスが記録されます
- **workerは初期化ディレクトリからのみ作成可能**: 記録されたディレクトリ以外からのworker作成は拒否されます
- **worktreeディレクトリからの作成禁止**: `worktree/`配下からのworker作成は禁止されます

```bash
# 正しい使用例
cd /project-A
tw init              # プロジェクトAで初期化
tw add feature-1     # ✅ 成功

cd /project-B
tw add feature-2     # ❌ 失敗（プロジェクトAで初期化されているため）

tw init              # プロジェクトBで新規初期化
tw add feature-2     # ✅ 成功
```

## 前提条件

- Go 1.19以降
- tmux
- git

## インストール

### go install を使用（推奨）

```bash
go install github.com/nakamasato/tmux-worker-manager@latest
ln -s $(which tmux-worker-manager) $(go env GOPATH)/bin/tw
```

### または、ソースからビルド

```bash
# リポジトリのクローン
git clone https://github.com/nakamasato/tmux-worker-manager.git
cd tmux-worker-manager

# ビルド
make build

# インストール
make install-user  # ~/.local/bin にインストール
# または
make install      # /usr/local/bin にインストール（sudo必要）
```

## 使用方法

### 基本的なワークフロー

```bash
# 1. tmuxセッションを初期化
tw init

# または、カスタム設定で初期化
tw init --command "claude --dangerously-skip-permissions" --worktree-prefix "work"

# 2. 新しいワーカーを作成
tw add issue-123

# 3. セッションに接続して作業開始
tw attach

# 4. 作業完了後、ワーカーを削除
tw remove issue-123

# 5. セッションを削除
tw destroy
```

### ワーカーの作成

```bash
# Issue番号でワーカー作成
tw add issue-123

# 機能名でワーカー作成
tw add feature-auth

# バグ修正でワーカー作成
tw add bug-login-fix
```

ワーカー作成時に自動的に以下が実行されます：
- git worktreeの作成
- tmux paneの作成
- 設定されたClaudeコマンドの実行

### ワーカー一覧の表示

```bash
tw list
```

出力例：
```
ID                   STATUS          WORKTREE PATH                  TMUX SESSION              PANE       CREATED
---------------------------------------------------------------------------------------------------------
issue-123            active          worktree/issue-123             myproject     %201       2024-01-15 10:30
feature-auth         inactive        worktree/feature-auth          myproject     %202       2024-01-15 09:15
```

### ワーカーの詳細状態確認

```bash
tw status issue-123
```

### ワーカーの削除

```bash
tw remove issue-123
```

### tmuxセッションの操作

```bash
# セッションに接続
tw attach

# セッションから切断（Ctrl+b d でも可能）
tw detach

# または直接tmuxコマンドで接続
tmux attach-session -t myproject
```

### 整合性チェックと修復

```bash
# worktreeとpaneの整合性をチェック
tw check

# 不整合を自動修復
tw repair
```

### 設定管理

#### 初期化時の設定

セッション初期化時にコマンドとworktreeパスを設定できます：

```bash
# デフォルト設定で初期化
tw init

# カスタム設定で初期化
tw init --command "claude --dangerously-skip-permissions" --worktree-prefix "work"

# 設定の確認
tw config
```

#### 初期化コマンドの変更

実行時のコマンド設定：

```bash
# 現在の設定を表示
tw config

# Claudeをpermissionsスキップで起動
tw config set "claude --dangerously-skip-permissions"

# 特定の設定を確認
tw config get
```

#### デフォルト設定

- **初期化コマンド**: `echo 'Hello, worker!'`
- **Worktreeプレフィックス**: `worktree`

#### 設定例

```bash
# Claude with bypassed permissions
tw config set "claude --dangerously-skip-permissions"

# npx でClaudeを使用
tw config set "npx claude"

# 開発サーバーを起動
tw config set "npm run dev"

# カスタムworktreeディレクトリ
tw init --worktree-prefix "workspace"

# 組み合わせ設定
tw init --command "npx claude" --worktree-prefix "features"
```

## ワーカーの構成

各ワーカーは専用のtmuxペインとして作成されます。`tw init` で初期セッションを作成し、`tw add` で新しいワーカーペインを追加します。

### tmuxセッション全体の構成例

複数のワーカーを追加した後の、tmuxセッション内のペイン配置例：

```
┌─────────────────┬─────────────────┐
│                 │                 │
│  プロジェクト    │  issue-123      │
│  ルート         │  (ワーカー1)     │
│                 │                 │
├─────────────────┼─────────────────┤
│                 │                 │
│  feature-auth   │  bug-fix        │
│  (ワーカー2)     │  (ワーカー3)     │
│                 │                 │
└─────────────────┴─────────────────┘
```

### ペインの構造

```
tmuxセッション（例: myproject）
├── ペイン0: プロジェクトルート（初期ペイン）
├── ペイン1: ワーカー1（例: issue-123）
├── ペイン2: ワーカー2（例: feature-auth）
└── ペインN: ワーカーN（例: bug-fix）
```

### ワーカーペインの特徴

- **独立したworktree**: 各ワーカーは専用のgit worktreeを持ちます
- **作業ディレクトリ**: 各ペインの作業ディレクトリは対応するworktreeに設定されます
- **初期化コマンド**: 設定されたコマンド（例: Claude AI）がペイン作成時に自動実行されます

## ディレクトリ構造

```
project-root/
├── worktree/
│   ├── issue-123/       # git worktree
│   ├── feature-auth/    # git worktree
│   └── bug-login-fix/   # git worktree
└── .tmux-workers.json   # ワーカー管理設定
```

## 設定ファイル

`.tmux-workers.json` にワーカー情報とコマンド設定が保存されます：

```json
{
  "workers": [
    {
      "id": "issue-123",
      "worktree_path": "workspace/issue-123",
      "tmux_session": "myproject",
      "window_index": 0,
      "pane_id": "%201",
      "pane_index": 1,
      "created_at": "2024-01-15T10:30:00Z",
      "status": "active"
    }
  ],
  "init_command": "claude --dangerously-skip-permissions",
  "worktree_prefix": "workspace",
  "project_path": "/Users/username/project-directory"
}
```

### 設定項目

- **workers**: ワーカー一覧
  - **pane_id**: tmux paneの安定したID (主要な識別子)
  - **pane_index**: 後方互換性のためのインデックス
- **init_command**: ワーカー作成時に実行するコマンド
- **worktree_prefix**: worktreeディレクトリのプレフィックス（デフォルト: "worktree"）
- **project_path**: セッションが初期化されたディレクトリのパス

## 開発者向け

### 開発環境のセットアップ

```bash
make setup
```

### 開発モードでの実行

```bash
# ワーカー追加をテスト
make dev ARGS="add test-issue"

# リスト表示をテスト
make dev ARGS="list"
```

### ビルドとテスト

```bash
# ビルド
make build

# 基本テスト実行
make test

# Go単体テスト実行
make test-unit

# シナリオベース統合テスト実行
make test-scenarios

# ベンチマークテスト実行
make test-bench

# 全テスト実行
make test-all

# クリーンアップ
make clean
```

## カスタマイズ

### 初期化コマンドの変更

コードを編集せずに、設定コマンドで簡単に変更できます：

```bash
# Claudeコマンドを変更
tw config set "claude --dangerously-skip-permissions"

# 異なるClaudeバージョンを使用
tw config set "npx claude@latest"

# ローカルのClaudeを使用
tw config set "/usr/local/bin/claude"

# 開発サーバーを起動
tw config set "npm run dev"

# Python環境をセットアップ
tw config set "python -m venv venv && source venv/bin/activate && pip install -r requirements.txt"
```

### ワーカーテンプレートの変更

tmuxペインのレイアウトやコマンドの詳細な調整は `main.go` の `addWorker` 関数内で可能です。

## トラブルシューティング

### tmux関連

#### セッションが見つからない・接続できない

```bash
# tmuxセッション一覧を確認
tmux list-sessions
tmux ls

# 特定のセッションの詳細を確認
tmux list-windows -t <session-name>
tmux list-panes -t <session-name>

# セッションの存在確認
tmux has-session -t <session-name>

# セッションを初期化
tw init

# 現在のプロジェクトディレクトリを確認
pwd
```

#### ペインの確認と操作

```bash
# 全ペインをID付きで表示
tmux list-panes -a -F "#{session_name}:#{window_index}.#{pane_index} #{pane_id} #{pane_title}"

# 特定セッションのペイン一覧
tmux list-panes -t <session-name> -F "#{pane_index} #{pane_id} #{pane_title}"

# ペインのタイトルを確認
tmux display-message -t <session-name> -p "#{pane_title}"

# 手動でペインを削除
tmux kill-pane -t <pane-id>
```

### git worktree関連

#### worktreeが作成されない・見つからない

```bash
# gitリポジトリ内で実行していることを確認
git status

# 既存のworktree一覧を確認
git worktree list

# worktreeの詳細情報を表示
git worktree list --verbose

# 特定のworktreeを手動で削除
git worktree remove <worktree-path>
git worktree remove --force <worktree-path>  # 強制削除

# 無効なworktreeをクリーンアップ
git worktree prune

# worktreeディレクトリの権限確認
ls -la worktree/

# 手動でworktreeディレクトリを作成
mkdir -p worktree
```

### 整合性の問題

#### worktreeとpaneの不整合

```bash
# 整合性をチェック
tw check

# 自動修復を実行
tw repair

# 手動で状態を確認
tw list
git worktree list
tmux list-panes -a
```

#### 設定ファイルの問題

設定は `.tmux-workers.json` に保存されるため、このファイルを削除すると設定がリセットされます：

```bash
# 設定ファイルを確認
cat .tmux-workers.json

# 設定を再確認
tw config

# 設定ファイルのバックアップ
cp .tmux-workers.json .tmux-workers.json.backup

# 設定ファイルを手動で編集（慎重に）
vim .tmux-workers.json
```

## ライセンス

MIT License
