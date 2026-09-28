# Shiori 使い方ガイド

`git pull`（または `git clone`）したあと、Shioriを使い始めるまでの手順と日常の使い方をまとめたものです。設計思想や方針は `README.md`、開発ルールは `AGENTS.md` を参照してください。

## 必要なもの

* Go 1.27.1 以上
* git

Cコンパイラは不要です（SQLiteドライバにピュアGo実装の `modernc.org/sqlite` を使っています）。

## 初回セットアップ（pull直後）

```bash
cd shiori

# 依存モジュールの取得（初回のみ）
go mod download

# ビルド
go build -o shiori ./cmd/shiori
```

データベースの準備は不要です。初回実行時に `data/bookmarks.db` が自動作成され、マイグレーションも自動適用されます。

```bash
./shiori list   # 空の一覧が表示されればOK
```

## CLIの使い方

リポジトリのルート（`data/` がある場所）で実行するのが基本です。別の場所から実行する場合は `--db` または環境変数 `SHIORI_DB` でDBファイルを指定します。

```bash
./shiori add https://example.com --title "Example" --comment "あとで読む" --tag dev --tag go
./shiori list
./shiori list --tag dev
./shiori list --favorites
./shiori show 1
./shiori search docker
./shiori search docker --tag dev
./shiori update 1 --title "新しいタイトル"
./shiori delete 1
./shiori favorite 1
./shiori unfavorite 1
./shiori tag add 1 docker
./shiori tag remove 1 docker
./shiori tag list
```

補足:

* `--tag` は `add` では繰り返し指定できます。タグは小文字・英数字・`-` `_` のみに正規化されます。
* `search` はURL・タイトル・コメントの全文検索（SQLite FTS5）で、`--tag` と組み合わせられます。`--favorites` との組み合わせはできません。
* `--json` を付けると機械可読JSONが出ます（例: `shiori search docker --json`）。スクリプトやAIエージェント向けです。
* 正常出力は stdout、エラーは stderr、終了コードは `0`=成功・`1`=実行時エラー・`2`=使い方エラーです。

## Web UIの使い方

```bash
./shiori serve
# http://localhost:8080/ をブラウザで開く
```

* ポート変更: `./shiori serve --addr localhost:9000` または環境変数 `SHIORI_ADDR`
* 一覧・検索・詳細・登録・編集・削除・タグ付け・お気に入り・タグ一覧が使えます。
* JavaScript不要のサーバー描画のみです。個人利用・localhost前提のため認証はありません。外部公開する場合は事前にセキュリティを見直してください。

## データの場所とバックアップ

* 既定: `data/bookmarks.db`（git管理対象外）
* 変更: `--db /path/to/db` または `SHIORI_DB=/path/to/db`
* バックアップは `shiori` を停止した状態で `data/bookmarks.db*` をコピーするだけです。

## 開発者向け

```bash
go test ./...          # 全テスト（一時DB使用、開発DBに触れない）
go vet ./...           # 静的検査
```

* SQLコード生成: `sqlc generate`（`sqlc.yaml`、`db/queries/`、`db/schema/`）。sqlcバイナリが別途必要です。
* マイグレーション: `db/migrations/` に goose 形式で追加し、アプリ起動時に自動適用されます（バイナリ内包）。**適用済みマイグレーションは編集せず、必ず新規ファイルで対応**してください。
* `internal/app`（アプリケーション層）が業務ロジックを持ち、CLI（`internal/cli`）とWeb（`internal/web`）は薄い変換層です。新機能は `app` に足し、両UIから使う形にしてください。

## トラブルシューティング

* `database is locked` が出る: SQLiteは単一プロセス利用が前提です。多重起動していないか確認してください。
* DBを消して最初からやり直したい: `shiori` を停止して `data/bookmarks.db*` を削除し、再度実行すると作り直されます。

## コンテナで動かす（実行用軽量イメージ）

開発用コンテナの `Dockerfile` とは別に、バイナリ実行専用の軽量イメージ用 `Dockerfile.run` を用意しています（マルチステージビルド＋最終 `scratch`）。

```bash
# イメージビルド
docker build -f Dockerfile.run -t shiori .
```

DBは名前付きボリューム（`/app/data`）に保存されます。コンテナを消してもデータは残ります。

```bash
# CLIを1回だけ実行
docker run --rm \
  -v shiori-data:/app/data \
  -e SHIORI_DB=/app/data/bookmarks.db \
  shiori list

# Web UIを常駐起動
docker run -d --name shiori \
  -p 8080:8080 \
  -v shiori-data:/app/data \
  -e SHIORI_DB=/app/data/bookmarks.db \
  -e SHIORI_ADDR=0.0.0.0:8080 \
  shiori serve
```

補足:

* コンテナ内では `0.0.0.0` で待ち受ける必要があります（`SHIORI_ADDR` または `--addr 0.0.0.0:8080`）。ホスト側への公開範囲は `-p 127.0.0.1:8080:8080` のように絞れます。
* bind mount を使う場合は非root実行（UID 65532）のため権限に注意してください。書き込めない場合は `--user $(id -u):$(id -g)` を付けて実行します。
* `pragma "PRAGMA journal_mode=WAL;": unable to open database file (14)` が出る場合は `/app/data` に書き込めていません。名前付きボリュームを古いイメージで作ったままの場合は、バックアップ後に `docker volume rm shiori-data` で作り直すと直ることがあります（初回作成時の所有権が引き継がれるため）。応急処置として `docker run --rm --user root -v shiori-data:/data alpine chown -R 65532:65532 /data` で所有権を直す方法もあります。
* バックアップは `docker stop shiori` 後に `docker cp shiori:/app/data/bookmarks.db ./` などで取り出せます。
