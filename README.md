# Shiori

自分用のシンプルなブックマークサービス。

URL、メモ、タグを保存し、CLIから登録・検索・管理できるようにする。将来的にはJavaScriptに依存しないシンプルなWeb UIを追加する。

## 方針

* 個人利用を前提とする
* まずCLIを完成させる
* データベースはSQLite
* SQLアクセスには`database/sql`とsqlcを使用する
* ORMは使用しない
* マイグレーションにはgooseを使用する
* 全文検索にはSQLite FTS5を使用する
* タグはフラットな手動タグのみとする
* 階層型タグは作らない
* Web UIはGoの`html/template`を使用する
* Web UIは通常のHTMLフォームとPRG（Post/Redirect/Get）を基本とする
* SPAやJavaScript依存のUIは作らない
* CLIとWeb UIでは同じアプリケーション層を利用する
* UIからSQLiteへ直接アクセスしない
* 依存関係は必要最小限にする
* 関数型コア・命令型シェルを可能な範囲で採用する
* RAGやAI機能などは初期実装に含めない

## 現在のCLI

予定している基本コマンドは以下。

```text
shiori add <URL>
shiori list
shiori show <ID>
shiori search <QUERY>
shiori update <ID>
shiori delete <ID>

shiori tag add <ID> <TAG>
shiori tag remove <ID> <TAG>
```

AIやスクリプトから扱いやすくするため、JSON出力も用意する。

```bash
shiori search docker --json
```

## ディレクトリ構成

```text
.
├── AGENTS.md
├── README.md
├── Dockerfile
├── opencode.json
├── go.mod
├── go.sum
├── cmd/
│   └── shiori/
├── internal/
│   └── db/
│       └── generated/
├── db/
│   ├── migrations/
│   └── queries/
└── data/
```

SQLiteのデータベースは以下に保存する。

```text
data/bookmarks.db
```

データベースファイルはGit管理対象外とする。

```gitignore
data/*.db
data/*.db-*
```

## データベース

SQLiteを使用する。

SQLコード生成にはsqlc、マイグレーションにはgooseを使用する。

想定している主要なテーブルは以下。

* `bookmarks`
* `tags`
* `bookmark_tags`

全文検索にはSQLite FTS5を使用する。

SQLiteへのアクセスはアプリケーション層から行い、CLIやWeb UIから直接SQLを実行しない。

## 開発環境

開発環境にはDockerを使用する。

現在の開発用コンテナはUbuntu 24.04をベースとし、シェルにはbashを使用する。

Dockerfileには開発に必要な最低限の基本パッケージだけを入れる。

```text
Ubuntu 24.04
├── bash
├── curl
├── git
├── ca-certificates
├── build-essential
├── pkg-config
└── sqlite3
```

以下のツールはDockerfileには含めず、永続コンテナを起動した後に必要に応じて手動でインストールする。

* mise
* Go
* Helix
* Starship
* OpenCode

これにより、Dockerfileを必要以上に複雑にせず、開発環境のツールはコンテナ内で自由に更新できる。

### Dockerfile

現在のDockerfileは以下。

```dockerfile
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && \
    apt-get install -y \
        ca-certificates \
        curl \
        git \
        build-essential \
        pkg-config \
        sqlite3 \
    && rm -rf /var/lib/apt/lists/*

RUN useradd \
    --create-home \
    --shell /bin/bash \
    dev

USER dev

WORKDIR /workspace/shiori

CMD ["/bin/bash"]
```

`WORKDIR`によってコンテナ内の作業ディレクトリは`/workspace/shiori`になる。

## Docker開発環境の使い方

### 1. Dockerイメージを作る

プロジェクトのルートで実行する。

```bash
docker build -t shiori-dev .
```

### 2. 初回だけコンテナを作成する

```bash
docker run -it \
    -p 8080:8080 \
    --name shiori-dev \
    --mount type=bind,src="$PWD",dst=/workspace/shiori \
    shiori-dev
```

プロジェクトディレクトリだけをコンテナ内の`/workspace/shiori`にマウントする。

Dockerのbind mountは、ホストとコンテナの間でファイルを共有する用途に適している。

### 3. コンテナから退出する

```bash
exit
```

この操作ではコンテナ自体は削除されない。

コンテナ内で手動インストールしたmise、Go、Helix、Starship、OpenCodeなどの状態も、コンテナを削除しない限り保持される。

### 4. 次回からコンテナを起動する

```bash
docker start -ai shiori-dev
```

`docker start`では停止済みコンテナを以前の状態のまま再開できる。

したがって、通常の開発では以下の流れになる。

```text
初回
  ↓
docker build
  ↓
docker run
  ↓
コンテナ内で開発ツールをセットアップ
  ↓
exit
  ↓
以降
  ↓
docker start -ai shiori-dev
```

### 5. コンテナを停止する

コンテナ内からは、

```bash
exit
```

で退出できる。

別のターミナルから停止する場合は、

```bash
docker stop shiori-dev
```

を使用する。

### 6. コンテナの状態を確認する

起動中のコンテナ：

```bash
docker ps
```

停止中のものも含めて確認：

```bash
docker ps -a
```

### 7. コンテナを削除する

```bash
docker rm shiori-dev
```

コンテナを削除すると、コンテナ内部にだけ保存されていたファイルや手動インストールした開発ツールは失われる。

一方、プロジェクトファイルはホスト側の`$PWD`をbind mountしているため、コンテナを削除してもプロジェクト自体は残る。

SQLiteのデータベースもプロジェクトディレクトリ内の

```text
data/bookmarks.db
```

に保存されるため、同様にホスト側に残る。

Dockerではコンテナの書き込みレイヤーはコンテナ削除時に失われるため、保持したいデータはbind mountやvolumeなどの外部ストレージに置く必要がある。

## コンテナ内での開発ツール

コンテナを初回作成したら、必要な開発ツールをコンテナ内でセットアップする。

例：

```bash
mise
Go
Helix
Starship
OpenCode
```

これらはDockerfileに固定せず、開発環境として必要になった時点でコンテナ内に導入する。

コンテナを削除しない限り、その状態は保持される。

## OpenCode

OpenCodeはコンテナ内で使用する。

基本的には、

```text
ホスト
└── Docker
    └── shiori-dev
        └── OpenCode
            └── /workspace/shiori
```

という構成にする。

OpenCodeからアクセスできるプロジェクト領域をコンテナ内の`/workspace/shiori`に限定することで、ホスト側のホームディレクトリやSSHキーなどをコンテナに直接公開しない。

Dockerソケットやホストのホームディレクトリなど、開発に不要なホスト資源はコンテナへマウントしない。

## Docker Compose

現時点ではDocker Composeを使用しない。

Shioriの開発環境は単一のコンテナで完結するため、Composeを導入する必要はない。

将来、データベースサーバーやその他の複数サービスが必要になった場合に検討する。

## 基本的な開発フロー

通常は以下の手順で開発する。

```bash
# ホスト側
cd ~/workspace/shiori

# 初回のみ
docker build -t shiori-dev .

docker run -it \
    --name shiori-dev \
    --mount type=bind,src="$PWD",dst=/workspace/shiori \
    shiori-dev
```

コンテナ内で開発ツールをセットアップした後は、

```bash
exit
```

で退出する。

次回以降は、

```bash
docker start -ai shiori-dev
```

で開発環境へ戻る。

コンテナ内では、

```bash
cd /workspace/shiori
```

がプロジェクトルートになる。

## Dockerイメージを作り直す

Dockerfileを変更した場合は、イメージを再ビルドする。

```bash
docker build -t shiori-dev .
```

既存の`shiori-dev`コンテナは古いイメージから作られたものなので、Dockerfileの変更を反映するにはコンテナも作り直す必要がある。

```bash
docker rm shiori-dev
docker build -t shiori-dev .
docker run -it \
    --name shiori-dev \
    --mount type=bind,src="$PWD",dst=/workspace/shiori \
    shiori-dev
```

この場合、コンテナ内に手動でインストールした開発ツールも作り直しになる。

そのため、Dockerfileを頻繁に変更する必要がない間は、コンテナをそのまま維持して使用する。

## Dockerイメージの確認

```bash
docker images
```

不要なイメージを削除する場合：

```bash
docker rmi shiori-dev
```

## 設計上の注意

Shioriは個人利用の小規模なサービスであるため、最初から大規模なWebアプリケーション構成にはしない。

特に以下は初期段階では導入しない。

* ORM
* SPA
* JavaScriptフレームワーク
* 複雑なHTTP API
* マイクロサービス
* RAG
* ベクトルデータベース
* 不要な認証基盤
* 複雑なタグ階層
* 不要な外部サービス

まずCLIとデータモデルを安定させ、その後必要になった機能だけを追加する。

## 将来のWeb UI

CLIが完成した後、必要であればWeb UIを追加する。

想定する構成は以下。

```text
ブラウザ
   ↓
Go HTTPサーバー
   ↓
アプリケーション層
   ↓
リポジトリ層
   ↓
SQLite
```

HTML生成にはGoの`html/template`を使用する。

基本的な操作は通常のHTMLフォームで行い、POST後はPRGを使用する。

JavaScriptを必須にしない。

CLIとWeb UIでビジネスロジックを共有し、インターフェースだけを分離する。

## 将来の拡張

必要になった場合のみ、以下を検討する。

* Web UI
* Basic認証
* ブックマーク本文の保存
* 外部コンテンツの取得
* より高度な全文検索
* AIによる検索・要約
* RAG
* VPSへのデプロイ

ただし、これらは初期実装の要件ではない。
