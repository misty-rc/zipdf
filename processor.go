package main

import (
	"archive/zip"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "golang.org/x/image/webp"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/signintech/gopdf"
)

// ProcessZip は1つのzipファイルを処理し、同名のPDFファイルを生成します。
func ProcessZip(zipPath string) error {
	// 一時ディレクトリの作成
	tempDir, err := os.MkdirTemp("", "zip2pdf-*")
	if err != nil {
		return fmt.Errorf("一時ディレクトリの作成に失敗しました: %w", err)
	}
	defer os.RemoveAll(tempDir) // 処理終了後に必ず削除

	// zipファイルを開く
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zipファイルの展開に失敗しました: %w", err)
	}
	defer r.Close()

	var extractedImages []string

	// zip内のファイルを走査して画像を抽出
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			// 一時ファイルとして保存
			extractedPath, err := extractFile(f, tempDir)
			if err != nil {
				return fmt.Errorf("ファイル %s の抽出に失敗しました: %w", f.Name, err)
			}
			extractedImages = append(extractedImages, extractedPath)
		}
	}

	if len(extractedImages) == 0 {
		return fmt.Errorf("zip内に有効な画像ファイルが見つかりませんでした")
	}

	// 元のファイル名順にソート（パスのベースネームで比較）
	sort.Slice(extractedImages, func(i, j int) bool {
		return filepath.Base(extractedImages[i]) < filepath.Base(extractedImages[j])
	})

	// PDF出力先パスの決定 (元のzipの名前の拡張子を .pdf に変更)
	pdfPath := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".pdf"
	
	err = generatePDF(extractedImages, pdfPath)
	if err != nil {
		return fmt.Errorf("PDFの生成に失敗しました: %w", err)
	}

	fmt.Printf("成功: %s -> %s (%d ページ)\n", filepath.Base(zipPath), filepath.Base(pdfPath), len(extractedImages))
	return nil
}

// extractFile はzip内のファイルを一時ディレクトリに展開し、そのパスを返します。
// WebP画像の場合は、gopdfがサポートしていないためPNGに変換して保存します。
func extractFile(f *zip.File, destDir string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	ext := strings.ToLower(filepath.Ext(f.Name))
	baseName := filepath.Base(f.Name)
	
	destPath := filepath.Join(destDir, baseName)
	
	// WebPの場合はPNGに変換して保存
	if ext == ".webp" {
		destPath = filepath.Join(destDir, strings.TrimSuffix(baseName, ext)+".png")
		
		img, _, err := image.Decode(rc)
		if err != nil {
			return "", fmt.Errorf("webpのデコードに失敗しました: %w", err)
		}

		// 16-bit PNGを回避するため、8-bitのRGBAに描画し直す
		bounds := img.Bounds()
		rgbaImg := image.NewRGBA(bounds)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				rgbaImg.Set(x, y, img.At(x, y))
			}
		}
		
		outFile, err := os.Create(destPath)
		if err != nil {
			return "", err
		}
		defer outFile.Close()
		
		err = png.Encode(outFile, rgbaImg)
		if err != nil {
			return "", fmt.Errorf("pngへのエンコードに失敗しました: %w", err)
		}
		
		return destPath, nil
	}

	// WebP以外（JPG, PNG等）はそのままコピー
	outFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	if err != nil {
		return "", err
	}

	return destPath, nil
}

// generatePDF は画像のリストからPDFを生成します。
func generatePDF(images []string, outputPath string) error {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})

	for _, imgPath := range images {
		// 画像のサイズを取得
		width, height, err := getImageDimensions(imgPath)
		if err != nil {
			return fmt.Errorf("画像サイズ取得エラー (%s): %w", filepath.Base(imgPath), err)
		}

		// 画像のサイズに合わせてページを追加（ピクセルからポイントへの変換の簡略化としてそのまま渡す。厳密にはDPI換算が必要ですが今回は簡易的にW,Hをそのまま使用）
		pdf.AddPageWithOption(gopdf.PageOption{
			PageSize: &gopdf.Rect{W: float64(width), H: float64(height)},
		})

		// ページに画像を描画
		err = pdf.Image(imgPath, 0, 0, &gopdf.Rect{W: float64(width), H: float64(height)})
		if err != nil {
			return fmt.Errorf("画像描画エラー (%s): %w", filepath.Base(imgPath), err)
		}
	}

	return pdf.WritePdf(outputPath)
}

// getImageDimensions は画像ファイルの幅と高さを取得します。
func getImageDimensions(imagePath string) (int, int, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}
