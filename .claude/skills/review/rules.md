# zipdf レビュールール（Go固有）

## 必須チェック項目

### ZIP処理
- `filepath.Base(f.Name)` でパストラバーサルを防いでいるか
- ZIPエントリが同名のとき `uniqueName` で重複排除しているか
- `uniqueName` が生成した名前も `seen` に登録しているか（元ファイル名との衝突防止）
- `zip.OpenReader` と抽出ファイルを `defer` で確実に閉じているか
- 一時ディレクトリを `defer os.RemoveAll` で確実に削除しているか
- `f.Mode().IsDir()` でディレクトリエントリをスキップしているか（`f.FileInfo()` は不要）

### WebP変換
- WebPデコード後に `image/draw` で8-bit RGBAへ変換しているか（gopdfが16-bit非対応のため必須）
- `_ "golang.org/x/image/webp"` のブランクインポートが残っているか

### JPEG再エンコード
- `reencodeAll` が `runtime.NumCPU()` をセマフォ上限にした並列処理になっているか
- `reencodeAsJPEG` がデコード時に寸法を取得して `imageInfo` に含めているか（`getImageDimensions` の二重呼び出しを避けるため）
- アルファチャンネルを白背景に合成してからエンコードしているか（JPEGはアルファ非対応）
- `--no-compress` 時は `quality=0` に変換されて再エンコードをスキップしているか

### imageInfo / generatePDF
- `generatePDF` が `[]imageInfo` を受け取り、内部で `getImageDimensions` を呼んでいないか
- `gopdf.Config{}` の初期PageSizeに依存していないか（`AddPageWithOption` で毎ページ上書きするため）

### エラー処理
- 全エラーを `%w` でラップして呼び出し元へ返しているか
- `log.Printf` でエラーを記録しつつ処理を継続する箇所と、`log.Fatalf` で即終了する箇所を使い分けているか
- `log.Printf` の末尾に `\n` を付けていないか（`log` が自動で改行を付与するため重複）

### PDF再圧縮（--recompress）
- `api.ExtractImagesFile(pdfPath, tempDir, nil, nil)` の引数順（inFile, outDir, selectedPages, conf）を守っているか
- 抽出画像のソートはアルファベット順で正しいか（pdfcpu はページ番号をゼロパディングするため）
- `--override` 時は同ディレクトリの一時ファイルへ書き込んでから `os.Rename` で上書きしているか（直接上書きしていないか）
- `--override` 単独（`--recompress` なし）を `log.Fatalf` で弾いているか

## 既知の制約（指摘不要）

- PDFページサイズはDPI変換なし（ピクセル値をポイントとして使用）— 意図的な設計
- `pdf.Image` がパスベースAPIのため `generatePDF` で各画像を1回openする — gopdf制約
- テストコードなし — スコープ外
