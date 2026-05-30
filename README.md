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
