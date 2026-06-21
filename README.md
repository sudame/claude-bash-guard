# claude-bash-guard

AI の git/gh 操作を統治するためのツール群。

- `bash-guard` — Claude Code の `PreToolUse` Bash フック。危険・非推奨なコマンドをブロック／確認する。
- `botpr` — GitHub App(bot) 名義で Pull Request を作るヘルパー。`gh pr create` の代わりに使う。

## bash-guard

`tool_input.command` を JSON で受け取り、以下のいずれかを返す:

- `exit 0` (stdout 空) — 通過
- `exit 2` (stderr にメッセージ) — ブロック
- `exit 0` + `permissionDecision: "ask"` JSON — 都度確認

### ルール

| # | 条件 | 結果 |
|---|---|---|
| 1 | `&&` / `;` でのコマンド連結（引用符内は除外） | block |
| 2 | `git -C ...` | block |
| 3 | `cd ...`（`aicd` を案内） | block |
| 4 | `gh api` + `-X` / `--method` （ただし下記例外） | **ask** |
| 4a | `gh api .../comments/<id>/replies -X POST` (PR レビューコメントへの返信) | allow |
| 5 | `aws ...` で `--profile` なし | block |

各ルールは設定ファイルの `disabled_rules` でグローバルに無効化できる。ルール ID は次のとおり:

| ルール | ID |
|---|---|
| 1 コマンド連結 | `chaining` |
| 2 `git -C` | `git_dash_c` |
| 3 `cd` | `cd` |
| 4 `gh api` 書き込み | `gh_api_write` |
| 5 `aws` `--profile` なし | `aws_no_profile` |

## botpr

「AI が出す PR は私(人間)がレビュー・Approve したい」を実現するためのヘルパー。GitHub は自分の PR を自分で Approve できないので、PR の author を「自分以外の identity」にする必要がある。`botpr` は **GitHub App(bot)** をその identity として使う。

仕組み:

1. macOS keychain に保存した GitHub App の秘密鍵(.pem)を読む
2. それで App JWT を署名し、installation access token を取得する（サーバー不要・ローカル完結）
3. `GH_TOKEN` にその token をセットして `gh pr create` を実行する → PR の author が `app[bot]` になる

commit の author/committer はローカルの git 設定のまま（あなた名義 ＋ Claude Code の co-author）。**PR の author だけが bot** になる。

引数はすべてそのまま `gh pr create` に渡される:

```sh
botpr --fill --base main
# ↑ 内部で `gh pr create --fill --base main` を bot token で実行
```

### セットアップ

#### 1. GitHub App を作る（一回だけ）

1. Settings → Developer settings → **GitHub Apps** → New GitHub App
2. 権限を最小限に: **Repository permissions → Contents = Read and write**, **Pull requests = Read and write**
3. **Webhook の "Active" のチェックを外す**（イベント受信は不要なのでサーバーもいらない）
4. 作成後、秘密鍵を生成して `.pem` をダウンロード
5. 対象アカウント/リポジトリに **Install**
6. `App ID`（App 設定ページ）と `Installation ID`（install 後の設定ページ URL `.../installations/<ID>` の数字）を控える

#### 2. 秘密鍵を keychain に保存

Keychain Access の GUI ではなく、**`security` コマンドで汎用パスワードとして**保存する（botpr は汎用パスワードを読む。.pem を GUI で import しようとすると失敗する）:

```sh
security add-generic-password -U -s claude-botpr -a pem -w "$(cat path/to/app.private-key.pem)"
```

`-s`(service) と `-a`(account) は設定ファイルの `keychain_service` / `keychain_account` と合わせる（未設定なら `claude-botpr` / `pem` が既定）。保存できたか確認:

```sh
security find-generic-password -s claude-botpr -a pem -w
```

`.pem` ファイルは保存後に削除してよい。

#### 3. 設定ファイルに App 情報を書く（下記参照）

## 設定ファイル

パスは次の順で解決:

1. 環境変数 `CLAUDE_BASH_GUARD_CONFIG`
2. `$XDG_CONFIG_HOME/claude-bash-guard.yaml`
3. `~/.config/claude-bash-guard.yaml`

```yaml
# botpr が installation token を発行するための GitHub App 情報。
botpr:
  app_id: 123456
  installation_id: 789012
  # keychain の場所（省略時は claude-botpr / pem）
  keychain_service: claude-botpr
  keychain_account: pem

# グローバルに無効化するルール ID の一覧（上記の表を参照）。未知の ID は無視される。
disabled_rules:
  - cd
  - aws_no_profile
```

`disabled_rules` に挙げたルールは常に無効になる。

## ビルド

```sh
make build   # → bin/bash-guard, bin/botpr
make test
```

## settings.json への組み込み（bash-guard）

```json
{
  "hooks": {
    "PreToolUse": [{
      "matcher": "Bash",
      "hooks": [{
        "type": "command",
        "command": "/absolute/path/to/bin/bash-guard"
      }]
    }]
  }
}
```

`botpr` は `bin/botpr` を `PATH` の通った場所に置く（または symlink）と AI から `botpr ...` で呼べる。
