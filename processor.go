package main

import (
	"archive/zip"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	_ "golang.org/x/image/webp"

	"github.com/signintech/gopdf"
)

type imageInfo struct {
	path          string
	width, height int
}

// ProcessZip は1つのzipファイルを処理し、同名のPDFファイルを生成します。
// quality が 1〜100 の場合、再エンコードを並列処理します。
func ProcessZip(zipPath string, quality int) error {
	tempDir, err := os.MkdirTemp("", "zip2pdf-*")
	if err != nil {
		return fmt.Errorf("一時ディレクトリの作成に失敗しました: %w", err)
	}
	defer os.RemoveAll(tempDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("zipファイルのオープンに失敗しました: %w", err)
	}
	defer r.Close()

	var extractedPaths []string
	seen := make(map[string]struct{})

	for _, f := range r.File {
		if f.Mode().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		extractedPath, err := extractFile(f, tempDir, seen)
		if err != nil {
			return fmt.Errorf("ファイル %s の抽出に失敗しました: %w", f.Name, err)
		}
		extractedPaths = append(extractedPaths, extractedPath)
	}

	if len(extractedPaths) == 0 {
		return fmt.Errorf("zip内に有効な画像ファイルが見つかりませんでした")
	}

	bases := make([]string, len(extractedPaths))
	for i, p := range extractedPaths {
		bases[i] = filepath.Base(p)
	}
	sort.Slice(extractedPaths, func(i, j int) bool {
		return bases[i] < bases[j]
	})

	var images []imageInfo
	if quality > 0 {
		images, err = reencodeAll(extractedPaths, quality)
		if err != nil {
			return err
		}
	} else {
		images = make([]imageInfo, len(extractedPaths))
		for i, p := range extractedPaths {
			w, h, err := getImageDimensions(p)
			if err != nil {
				return fmt.Errorf("画像サイズの取得に失敗しました (%s): %w", filepath.Base(p), err)
			}
			images[i] = imageInfo{path: p, width: w, height: h}
		}
	}

	pdfPath := strings.TrimSuffix(zipPath, filepath.Ext(zipPath)) + ".pdf"
	if err := generatePDF(images, pdfPath); err != nil {
		return fmt.Errorf("PDFの生成に失敗しました: %w", err)
	}

	fmt.Printf("成功: %s -> %s (%d ページ)\n", filepath.Base(zipPath), filepath.Base(pdfPath), len(images))
	return nil
}

// reencodeAll は画像を runtime.NumCPU() 並列でJPEG再エンコードします。
func reencodeAll(paths []string, quality int) ([]imageInfo, error) {
	type result struct {
		info imageInfo
		err  error
	}
	results := make([]result, len(paths))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup

	for i, p := range paths {
		wg.Add(1)
		i, p := i, p
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := reencodeAsJPEG(p, quality)
			results[i] = result{info, err}
		}()
	}
	wg.Wait()

	infos := make([]imageInfo, len(paths))
	for i, r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("JPEG再エンコードに失敗しました (%s): %w", filepath.Base(paths[i]), r.err)
		}
		infos[i] = r.info
	}
	return infos, nil
}

// extractFile はzip内のファイルを一時ディレクトリに展開し、そのパスを返します。
// WebP画像の場合は、gopdfがサポートしていないためPNGに変換して保存します。
func extractFile(f *zip.File, destDir string, seen map[string]struct{}) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("zipエントリのオープンに失敗しました: %w", err)
	}
	defer rc.Close()

	ext := strings.ToLower(filepath.Ext(f.Name))
	baseName := uniqueName(filepath.Base(f.Name), seen)

	if ext == ".webp" {
		stem := strings.TrimSuffix(baseName, ext)
		return extractWebP(rc, destDir, stem)
	}

	destPath := filepath.Join(destDir, baseName)
	outFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("出力ファイルの作成に失敗しました: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, rc); err != nil {
		return "", fmt.Errorf("ファイルのコピーに失敗しました: %w", err)
	}
	return destPath, nil
}

// extractWebP はWebP画像をデコードし、8-bit RGBAのPNGとして保存します。
func extractWebP(rc io.Reader, destDir, stem string) (string, error) {
	img, _, err := image.Decode(rc)
	if err != nil {
		return "", fmt.Errorf("WebPのデコードに失敗しました: %w", err)
	}

	// gopdfが16-bit PNGを扱えないため8-bit RGBAに変換する
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)

	destPath := filepath.Join(destDir, stem+".png")
	outFile, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("出力ファイルの作成に失敗しました: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, rgba); err != nil {
		return "", fmt.Errorf("PNGへのエンコードに失敗しました: %w", err)
	}
	return destPath, nil
}

// reencodeAsJPEG は画像をJPEGで再エンコードし、imageInfo（パス・寸法）を返します。
// デコード時に寸法を取得するため getImageDimensions の呼び出しを省きます。
// PNGのアルファチャンネルは白背景に合成してからエンコードします。
func reencodeAsJPEG(imgPath string, quality int) (imageInfo, error) {
	var src image.Image
	{
		f, err := os.Open(imgPath)
		if err != nil {
			return imageInfo{}, fmt.Errorf("ファイルのオープンに失敗しました: %w", err)
		}
		src, _, err = image.Decode(f)
		f.Close()
		if err != nil {
			return imageInfo{}, fmt.Errorf("画像のデコードに失敗しました: %w", err)
		}
	}

	// JPEGはアルファ非対応のため白背景に合成する
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.White, image.Point{}, draw.Src)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)

	jpegPath := strings.TrimSuffix(imgPath, filepath.Ext(imgPath)) + "_q.jpg"
	out, err := os.Create(jpegPath)
	if err != nil {
		return imageInfo{}, fmt.Errorf("出力ファイルの作成に失敗しました: %w", err)
	}
	defer out.Close()

	if err := jpeg.Encode(out, dst, &jpeg.Options{Quality: quality}); err != nil {
		return imageInfo{}, fmt.Errorf("JPEGエンコードに失敗しました: %w", err)
	}

	return imageInfo{
		path:   jpegPath,
		width:  bounds.Dx(),
		height: bounds.Dy(),
	}, nil
}

// generatePDF は imageInfo のリストからPDFを生成します。
// ページサイズは各画像のピクセルサイズをそのままポイント単位として使用します。
func generatePDF(images []imageInfo, outputPath string) error {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{})

	for _, img := range images {
		rect := &gopdf.Rect{W: float64(img.width), H: float64(img.height)}
		pdf.AddPageWithOption(gopdf.PageOption{PageSize: rect})
		if err := pdf.Image(img.path, 0, 0, rect); err != nil {
			return fmt.Errorf("画像の描画に失敗しました (%s): %w", filepath.Base(img.path), err)
		}
	}

	return pdf.WritePdf(outputPath)
}

// getImageDimensions は画像ファイルの幅と高さを取得します。
func getImageDimensions(imagePath string) (int, int, error) {
	f, err := os.Open(imagePath)
	if err != nil {
		return 0, 0, fmt.Errorf("ファイルのオープンに失敗しました: %w", err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("画像設定のデコードに失敗しました: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}

// ProcessPDF は既存のPDFから画像を抽出し、再エンコードして新しいPDFを生成します。
// override=true の場合は元ファイルを上書きし、false の場合は _compressed サフィックスを付けて出力します。
func ProcessPDF(pdfPath string, quality int, override bool) error {
	tempDir, err := os.MkdirTemp("", "pdf2pdf-*")
	if err != nil {
		return fmt.Errorf("一時ディレクトリの作成に失敗しました: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("  画像抽出中: %s\n", filepath.Base(pdfPath))
	imagePaths, err := extractPDFImagesDirect(pdfPath, tempDir)
	if err != nil {
		return err
	}
	fmt.Printf("  %d 枚抽出完了、再エンコード中...\n", len(imagePaths))

	var images []imageInfo
	if quality > 0 {
		images, err = reencodeAll(imagePaths, quality)
		if err != nil {
			return err
		}
	} else {
		images = make([]imageInfo, len(imagePaths))
		for i, p := range imagePaths {
			w, h, err := getImageDimensions(p)
			if err != nil {
				return fmt.Errorf("画像サイズの取得に失敗しました (%s): %w", filepath.Base(p), err)
			}
			images[i] = imageInfo{path: p, width: w, height: h}
		}
	}

	ext := filepath.Ext(pdfPath)
	outputPath := strings.TrimSuffix(pdfPath, ext) + "_compressed" + ext
	if override {
		// 入力ファイルと同じパスへ上書きするため、同ディレクトリの一時ファイルに書き込んでからリネーム
		tmp, err := os.CreateTemp(filepath.Dir(pdfPath), ".zipdf-tmp-*.pdf")
		if err != nil {
			return fmt.Errorf("一時ファイルの作成に失敗しました: %w", err)
		}
		tmp.Close()
		outputPath = tmp.Name()
	}

	if err := generatePDF(images, outputPath); err != nil {
		if override {
			os.Remove(outputPath)
		}
		return fmt.Errorf("PDFの生成に失敗しました: %w", err)
	}

	if override {
		if err := os.Rename(outputPath, pdfPath); err != nil {
			os.Remove(outputPath)
			return fmt.Errorf("ファイルの上書きに失敗しました: %w", err)
		}
		fmt.Printf("成功: %s (上書き, %d ページ)\n", filepath.Base(pdfPath), len(images))
	} else {
		fmt.Printf("成功: %s -> %s (%d ページ)\n", filepath.Base(pdfPath), filepath.Base(outputPath), len(images))
	}
	return nil
}


// uniqueName はZIP内で同名ファイルが重複した場合に一意なファイル名を返します。
// 生成した名前も seen に登録するため、元ファイル名との衝突も防止します。
func uniqueName(name string, seen map[string]struct{}) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	candidate := name
	for i := 1; ; i++ {
		if _, exists := seen[candidate]; !exists {
			seen[candidate] = struct{}{}
			return candidate
		}
		candidate = fmt.Sprintf("%s_%d%s", stem, i, ext)
	}
}
