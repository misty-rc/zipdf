# zipdf レビュールール（Go固有）

## 必須チェック項目

### ZIP処理
- `filepath.Base(f.Name)` でパストラバーサルを防いでいるか
- ZIPエントリが同名のとき `uniqueName` で重複排除しているか
- `zip.OpenReader` と抽出ファイルを `defer` で確実に閉じているか
- 一時ディレクトリを `defer os.RemoveAll` で確実に削除しているか

### WebP変換
- WebPデコード後に `image/draw` で8-bit RGBAへ変換しているか（gopdfが16-bit非対応のため必須）
- `_ "golang.org/x/image/webp"` のブランクインポートが残っているか

### gopdf
- `pdf.AddPageWithOption` でページサイズを画像ごとに設定しているか
- `gopdf.Config{}` の初期PageSizeに依存していないか（各ページが上書きするため無意味）

### エラー処理
- 全エラーを `%w` でラップして呼び出し元へ返しているか
- `log.Printf` でエラーを記録しつつ処理を継続する箇所と、`log.Fatalf` で即終了する箇所を使い分けているか

## 既知の制約（指摘不要）

- PDFページサイズはDPI変換なし（ピクセル値をポイントとして使用）— 意図的な設計
- テストコードなし — スコープ外
