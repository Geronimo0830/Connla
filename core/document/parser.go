package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
	"github.com/pkg/errors"

	"github.com/usememos/memos/markdown"
)

const (
	DefaultMaxInputBytes  = 8 << 20
	TextParserVersion     = "text-v1"
	MarkdownParserVersion = "markdown-v1"
	PDFParserVersion      = "pdf-v1"
	DOCXParserVersion     = "docx-v1"
	XLSXParserVersion     = "xlsx-v1"
	maxPDFPages           = 500
	maxPDFTextBytes       = 16 << 20
)

var (
	ErrUnsupportedFormat  = errors.New("unsupported document format")
	ErrInvalidUTF8        = errors.New("document is not valid UTF-8")
	ErrBinaryContent      = errors.New("document contains binary content")
	ErrInputTooLarge      = errors.New("document exceeds parser size limit")
	ErrInvalidPDF         = errors.New("PDF is malformed or unreadable")
	ErrPDFPageLimit       = errors.New("PDF exceeds page limit")
	ErrPDFTextTooLarge    = errors.New("PDF extracted text exceeds size limit")
	ErrInvalidOffice      = errors.New("Office document is malformed or unreadable")
	ErrOfficeArchiveLimit = errors.New("Office document exceeds archive limits")
	ErrOfficeContentLimit = errors.New("Office document exceeds content limits")
	ErrTimeout            = errors.New("document parsing timed out")
)

// Result is deterministic, replaceable output derived from one source file.
type Result struct {
	PlainText      string
	StructuredJSON string
	ContentHash    string
	Extractor      string
	Warnings       []string
}

// Parser parses supported documents without mutating their source bytes.
type Parser interface {
	Parse(ctx context.Context, extension, mediaType string, input []byte) (*Result, error)
}

type parser struct {
	markdownService markdown.Service
	maxInputBytes   int
}

// NewParser creates the deterministic document parser.
func NewParser(markdownService markdown.Service, maxInputBytes int) Parser {
	if maxInputBytes <= 0 {
		maxInputBytes = DefaultMaxInputBytes
	}
	return &parser{markdownService: markdownService, maxInputBytes: maxInputBytes}
}

// ErrorCode converts parser errors to stable, safe persistence codes.
func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrUnsupportedFormat):
		return "UNSUPPORTED_FORMAT"
	case errors.Is(err, ErrInvalidUTF8):
		return "INVALID_UTF8"
	case errors.Is(err, ErrBinaryContent):
		return "BINARY_CONTENT"
	case errors.Is(err, ErrInputTooLarge):
		return "INPUT_TOO_LARGE"
	case errors.Is(err, ErrInvalidPDF):
		return "INVALID_PDF"
	case errors.Is(err, ErrPDFPageLimit):
		return "PDF_PAGE_LIMIT"
	case errors.Is(err, ErrPDFTextTooLarge):
		return "PDF_TEXT_TOO_LARGE"
	case errors.Is(err, ErrInvalidOffice):
		return "INVALID_OFFICE_DOCUMENT"
	case errors.Is(err, ErrOfficeArchiveLimit):
		return "OFFICE_ARCHIVE_LIMIT"
	case errors.Is(err, ErrOfficeContentLimit):
		return "OFFICE_CONTENT_LIMIT"
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "TIMEOUT"
	default:
		return "PARSE_FAILED"
	}
}

// ParserVersion returns the deterministic parser identity selected for an extension.
func ParserVersion(extension string) string {
	switch strings.ToLower(extension) {
	case ".txt":
		return TextParserVersion
	case ".md", ".markdown":
		return MarkdownParserVersion
	case ".pdf":
		return PDFParserVersion
	case ".docx":
		return DOCXParserVersion
	case ".xlsx":
		return XLSXParserVersion
	default:
		return "unsupported-v1"
	}
}

func (p *parser) Parse(ctx context.Context, extension, mediaType string, input []byte) (*Result, error) {
	if err := parserContextError(ctx); err != nil {
		return nil, err
	}
	if len(input) > p.maxInputBytes {
		return nil, ErrInputTooLarge
	}
	extension = strings.ToLower(extension)
	if extension == ".pdf" {
		if mediaType != "application/pdf" {
			return nil, ErrUnsupportedFormat
		}
		return p.parsePDF(ctx, input)
	}
	if extension == ".docx" {
		if mediaType != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
			return nil, ErrUnsupportedFormat
		}
		return p.parseDOCX(ctx, input)
	}
	if extension == ".xlsx" {
		if mediaType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
			return nil, ErrUnsupportedFormat
		}
		return p.parseXLSX(ctx, input)
	}
	if !supportedTextDocument(extension, mediaType) {
		return nil, ErrUnsupportedFormat
	}
	if !utf8.Valid(input) {
		return nil, ErrInvalidUTF8
	}
	if bytes.IndexByte(input, 0) >= 0 {
		return nil, ErrBinaryContent
	}

	normalized, warnings := normalizeUTF8Text(input)
	plainText := normalized
	extractor := TextParserVersion
	format := "text"
	if extension == ".md" || extension == ".markdown" {
		if p.markdownService == nil {
			return nil, errors.New("markdown service is unavailable")
		}
		var err error
		plainText, err = p.markdownService.ExtractPlainText([]byte(normalized))
		if err != nil {
			return nil, errors.Wrap(err, "failed to extract markdown text")
		}
		extractor = MarkdownParserVersion
		format = "markdown"
		warnings = append(warnings, "MARKDOWN_FORMATTING_REMOVED")
	}
	if err := parserContextError(ctx); err != nil {
		return nil, err
	}

	plainText = strings.TrimSpace(plainText)
	hash := sha256.Sum256([]byte(plainText))
	structured, err := json.Marshal(struct {
		Version  int      `json:"version"`
		Format   string   `json:"format"`
		Warnings []string `json:"warnings"`
	}{Version: 1, Format: format, Warnings: warnings})
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode parser metadata")
	}
	return &Result{
		PlainText:      plainText,
		StructuredJSON: string(structured),
		ContentHash:    hex.EncodeToString(hash[:]),
		Extractor:      extractor,
		Warnings:       warnings,
	}, nil
}

type pdfPage struct {
	Page int    `json:"page"`
	Text string `json:"text"`
}

func (*parser) parsePDF(ctx context.Context, input []byte) (result *Result, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = ErrInvalidPDF
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		return nil, errors.Wrap(ErrInvalidPDF, err.Error())
	}
	pageCount := reader.NumPage()
	if pageCount <= 0 {
		return nil, ErrInvalidPDF
	}
	if pageCount > maxPDFPages {
		return nil, ErrPDFPageLimit
	}

	pages := make([]pdfPage, 0, pageCount)
	plainPages := make([]string, 0, pageCount)
	emptyPages := make([]int, 0)
	totalTextBytes := 0
	for pageNumber := 1; pageNumber <= pageCount; pageNumber++ {
		if err := parserContextError(ctx); err != nil {
			return nil, err
		}
		page := reader.Page(pageNumber)
		if page.V.IsNull() {
			return nil, ErrInvalidPDF
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return nil, errors.Wrap(ErrInvalidPDF, err.Error())
		}
		text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
		totalTextBytes += len(text)
		if totalTextBytes > maxPDFTextBytes {
			return nil, ErrPDFTextTooLarge
		}
		pages = append(pages, pdfPage{Page: pageNumber, Text: text})
		if text == "" {
			emptyPages = append(emptyPages, pageNumber)
		} else {
			plainPages = append(plainPages, text)
		}
	}

	warnings := []string{}
	if len(emptyPages) > 0 {
		warnings = append(warnings, "PDF_EMPTY_OR_SCANNED_PAGES")
	}
	if len(emptyPages) == pageCount {
		warnings = append(warnings, "PDF_NO_EXTRACTABLE_TEXT")
	}
	plainText := strings.Join(plainPages, "\n\n")
	hash := sha256.Sum256([]byte(plainText))
	structured, err := json.Marshal(struct {
		Version    int       `json:"version"`
		Format     string    `json:"format"`
		PageCount  int       `json:"pageCount"`
		Pages      []pdfPage `json:"pages"`
		EmptyPages []int     `json:"emptyPages,omitempty"`
		Warnings   []string  `json:"warnings"`
	}{Version: 1, Format: "pdf", PageCount: pageCount, Pages: pages, EmptyPages: emptyPages, Warnings: warnings})
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode PDF metadata")
	}
	return &Result{
		PlainText:      plainText,
		StructuredJSON: string(structured),
		ContentHash:    hex.EncodeToString(hash[:]),
		Extractor:      PDFParserVersion,
		Warnings:       warnings,
	}, nil
}

func supportedTextDocument(extension, mediaType string) bool {
	switch extension {
	case ".txt":
		return mediaType == "text/plain"
	case ".md", ".markdown":
		return mediaType == "text/markdown" || mediaType == "text/plain"
	default:
		return false
	}
}

func normalizeUTF8Text(input []byte) (string, []string) {
	warnings := []string{}
	if bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) {
		input = input[3:]
		warnings = append(warnings, "UTF8_BOM_REMOVED")
	}
	text := string(input)
	if strings.Contains(text, "\r") {
		text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
		warnings = append(warnings, "NEWLINES_NORMALIZED")
	}
	return text, warnings
}

func parserContextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(ErrTimeout, err.Error())
	}
	return nil
}
