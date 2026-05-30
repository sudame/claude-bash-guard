# claude-bash-guard

Claude Code の `PreToolUse` Bash フック。`tool_input.command` を JSON で受け取り、以下のいずれかを返す:

- `exit 0` (stdout 空) — 通過
- `exit 2` (stderr にメッセージ) — ブロック
- `exit 0` + `permissionDecision: "ask"` JSON — 都度確認

## ルール

| # | 条件 | 結果 |
|---|---|---|
| 1 | `&&` / `;` でのコマンド連結（引用符内は除外） | block |
| 2 | `git -C ...` | block |
| 3 | `cd ...`（`aicd` を案内） | block |
| 4 | `gh api` + `-X` / `--method` （ただし下記例外） | **ask** |
| 4a | `gh api .../comments/<id>/replies -X POST` (PR レビューコメントへの返信) | allow |
| 5 | `aws ...` で `--profile` なし | block |
| 6 | `gh pr create` でアクティブな gh アカウントが設定値と不一致 | block（設定時のみ） |

ルール 6 は設定ファイル（下記）で `account` を指定したときだけ有効。アクティブアカウントは `gh auth status --active` で判定する。

各ルールは設定ファイルの `disabled_rules` でグローバルに無効化できる。ルール ID は次のとおり:

| ルール | ID |
|---|---|
| 1 コマンド連結 | `chaining` |
| 2 `git -C` | `git_dash_c` |
| 3 `cd` | `cd` |
| 4 `gh api` 書き込み | `gh_api_write` |
| 5 `aws` `--profile` なし | `aws_no_profile` |
| 6 `gh pr create` アカウント | `gh_pr_create` |

## 設定ファイル

`gh pr create` のアカウントチェックは設定ファイルで制御する。パスは次の順で解決:

1. 環境変数 `CLAUDE_BASH_GUARD_CONFIG`
2. `$XDG_CONFIG_HOME/claude-bash-guard.yaml`
3. `~/.config/claude-bash-guard.yaml`

```yaml
# gh pr create に使うべき GitHub アカウント。未設定/空ならこのチェックは無効。
account: sudame-bot

# 以下のパス配下（プレフィックス一致）では gh pr create のチェックをスキップ。
# sudame-bot を強制しないリポジトリ用。
exclude_paths:
  - /Users/sudame/oss

# グローバルに無効化するルール ID の一覧（上記の表を参照）。未知の ID は無視される。
disabled_rules:
  - cd
  - aws_no_profile
```

設定ファイルが無い・`account` が空の場合、ルール 6 は何もしない（通過）。除外判定は Claude Code フックの `cwd` を使う。`disabled_rules` に挙げたルールは `cwd` に関係なく常に無効になる。

## ビルド

```sh
make build   # → bin/bash-guard
make test
```

## settings.json への組み込み

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
