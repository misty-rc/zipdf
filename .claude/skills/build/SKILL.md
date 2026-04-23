---
name: build
description: zipdf をビルドする。「ビルド」「build」「コンパイル」「exe を作って」などのキーワードが出たら必ずこのスキルを使う。
---

# ビルド手順

## コマンド

```bash
go build -o zipdf.exe .
```

## 確認

```bash
ls -lh zipdf.exe
./zipdf --help
```

## エラー時

```bash
# 依存関係の不整合がある場合
go mod tidy
go build -o zipdf.exe .
```
