# zipdf

A command-line tool that merges images inside ZIP archives into a single PDF, sorted by filename.

## Purpose

- Convert images (jpg/jpeg/png/webp) inside a ZIP into a PDF, sorted by filename
- Process multiple ZIPs in bulk — one ZIP produces one PDF
- Designed for command-line use

## Installation

```bash
git clone https://github.com/misty-rc/zipdf
cd zipdf
go build -o zipdf.exe .
```

## Usage

```bash
# Process all *.zip files in the specified directory (JPEG re-encode at quality 85)
./zipdf -i ./archive

# Set a custom quality
./zipdf -i ./archive -q 70

# No re-encoding (embed original images as-is)
./zipdf -i ./archive --no-compress

# Process the current directory
./zipdf

# Re-compress an existing PDF (outputs <name>_compressed.pdf)
./zipdf --recompress -i ./pdfs -q 70

# Re-compress and overwrite the original
./zipdf --recompress --override -i ./pdfs -q 70
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `-i`, `--input` | `.` (current dir) | Directory to process |
| `-q`, `--quality` | `85` | JPEG re-encode quality (1–100) |
| `--no-compress` | `false` | Disable re-encoding; embed original images |
| `--recompress` | `false` | Re-encode embedded images in existing PDFs |
| `--override` | `false` | Overwrite the original file when using `--recompress` (cannot be used alone) |

## Supported Formats

`.jpg` / `.jpeg` / `.png` / `.webp`

## Output

Each ZIP produces `<zip-name>.pdf` in the same directory as the ZIP file.

## Tech Stack

- Go
- [gopdf](https://github.com/signintech/gopdf) — PDF generation
- [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) — WebP decoding
