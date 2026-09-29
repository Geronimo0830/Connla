package document

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/usememos/memos/markdown"
)

func TestParserTXTAndMarkdown(t *testing.T) {
	parser := NewParser(markdown.NewService(markdown.WithTagExtension(), markdown.WithMentionExtension()), 1024)

	txt, err := parser.Parse(context.Background(), ".txt", "text/plain", []byte("\xef\xbb\xbfline one\r\nline two\r"))
	require.NoError(t, err)
	require.Equal(t, "line one\nline two", txt.PlainText)
	require.Equal(t, TextParserVersion, txt.Extractor)
	require.Equal(t, []string{"UTF8_BOM_REMOVED", "NEWLINES_NORMALIZED"}, txt.Warnings)
	require.Contains(t, txt.StructuredJSON, `"warnings":["UTF8_BOM_REMOVED","NEWLINES_NORMALIZED"]`)

	md, err := parser.Parse(context.Background(), ".md", "text/markdown", []byte("# Title\r\n\r\n- one **bold**\r\n- two\r\n\r\n```go\nkept()\n```"))
	require.NoError(t, err)
	require.Equal(t, "Title\none bold\ntwo\nkept()", md.PlainText)
	require.Equal(t, MarkdownParserVersion, md.Extractor)
	require.Equal(t, []string{"NEWLINES_NORMALIZED", "MARKDOWN_FORMATTING_REMOVED"}, md.Warnings)
}

func TestParserPDFPageAwareText(t *testing.T) {
	parser := NewParser(markdown.NewService(), DefaultMaxInputBytes)
	input, err := os.ReadFile("testdata/two-page-text.pdf")
	require.NoError(t, err)

	result, err := parser.Parse(context.Background(), ".pdf", "application/pdf", input)
	require.NoError(t, err)
	require.Equal(t, PDFParserVersion, result.Extractor)
	require.Contains(t, result.PlainText, "Page one: source-linked knowledge.")
	require.Contains(t, result.PlainText, "Page two: durable notes.")
	require.Contains(t, result.StructuredJSON, `"pageCount":2`)
	require.Contains(t, result.StructuredJSON, `"page":1`)
	require.Contains(t, result.StructuredJSON, `"page":2`)
	require.Empty(t, result.Warnings)
}

func TestParserPDFEmptyAndMalformed(t *testing.T) {
	parser := NewParser(markdown.NewService(), DefaultMaxInputBytes)
	empty, err := os.ReadFile("testdata/empty-page.pdf")
	require.NoError(t, err)

	result, err := parser.Parse(context.Background(), ".pdf", "application/pdf", empty)
	require.NoError(t, err)
	require.Empty(t, result.PlainText)
	require.Equal(t, []string{"PDF_EMPTY_OR_SCANNED_PAGES", "PDF_NO_EXTRACTABLE_TEXT"}, result.Warnings)
	require.Contains(t, result.StructuredJSON, `"emptyPages":[1]`)

	malformed, err := os.ReadFile("testdata/malformed.pdf")
	require.NoError(t, err)
	_, err = parser.Parse(context.Background(), ".pdf", "application/pdf", malformed)
	require.ErrorIs(t, err, ErrInvalidPDF)
	require.Equal(t, "INVALID_PDF", ErrorCode(err))
}

func TestParserDOCXStructureAndTables(t *testing.T) {
	parser := NewParser(markdown.NewService(), DefaultMaxInputBytes)
	input := officeTestArchive(t, map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml": `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
			<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Knowledge systems</w:t></w:r></w:p>
			<w:p><w:r><w:t>Capture</w:t><w:tab/><w:t>before organizing.</w:t></w:r></w:p>
			<w:tbl><w:tr><w:tc><w:p><w:r><w:t>Term</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Meaning</w:t></w:r></w:p></w:tc></w:tr>
			<w:tr><w:tc><w:p><w:r><w:t>Source</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Immutable original</w:t></w:r></w:p></w:tc></w:tr></w:tbl>
		</w:body></w:document>`,
	})

	result, err := parser.Parse(context.Background(), ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", input)
	require.NoError(t, err)
	require.Equal(t, DOCXParserVersion, result.Extractor)
	require.Contains(t, result.PlainText, "Knowledge systems")
	require.Contains(t, result.PlainText, "Term\tMeaning")
	require.Contains(t, result.StructuredJSON, `"locator":"paragraph:1"`)
	require.Contains(t, result.StructuredJSON, `"style":"Heading1"`)
	require.Contains(t, result.StructuredJSON, `"locator":"table:1"`)
	require.Contains(t, result.StructuredJSON, `"locator":"table:1:R2C1","row":2,"column":1,"text":"Source"`)
}

func TestParserXLSXSheetsCellsAndFormulaData(t *testing.T) {
	parser := NewParser(markdown.NewService(), DefaultMaxInputBytes)
	input := officeTestArchive(t, map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"xl/workbook.xml": `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>
			<sheet name="Summary" sheetId="1" r:id="rId1"/><sheet name="Notes" sheetId="2" r:id="rId2"/>
		</sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rId1" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Target="worksheets/sheet2.xml"/>
		</Relationships>`,
		"xl/sharedStrings.xml": `<sst><si><t>Shared note</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row r="1">
			<c r="A1" t="inlineStr"><is><t>Metric</t></is></c><c r="B1"><v>2</v></c><c r="C1"><f>B1*2</f><v>999</v></c>
		</row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c></row></sheetData></worksheet>`,
	})

	result, err := parser.Parse(context.Background(), ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", input)
	require.NoError(t, err)
	require.Equal(t, XLSXParserVersion, result.Extractor)
	require.Contains(t, result.PlainText, "[Summary]")
	require.Contains(t, result.PlainText, "C1: =B1*2 [cached: 999]")
	require.Contains(t, result.PlainText, "[Notes]")
	require.Contains(t, result.PlainText, "A1: Shared note")
	require.Contains(t, result.StructuredJSON, `"name":"Summary"`)
	require.Contains(t, result.StructuredJSON, `"reference":"C1","value":"999","formula":"B1*2"`)
}

func TestParserOfficeFailuresAndLegacyFormats(t *testing.T) {
	parser := NewParser(markdown.NewService(), DefaultMaxInputBytes)

	_, err := parser.Parse(context.Background(), ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("PKbroken"))
	require.ErrorIs(t, err, ErrInvalidOffice)
	require.Equal(t, "INVALID_OFFICE_DOCUMENT", ErrorCode(err))

	tooManySheets := strings.Builder{}
	tooManySheets.WriteString(`<workbook xmlns:r="urn:r"><sheets>`)
	for index := 1; index <= maxXLSXSheets+1; index++ {
		tooManySheets.WriteString(`<sheet name="S` + strconv.Itoa(index) + `" r:id="r` + strconv.Itoa(index) + `"/>`)
	}
	tooManySheets.WriteString(`</sheets></workbook>`)
	input := officeTestArchive(t, map[string]string{
		"[Content_Types].xml":        `<Types/>`,
		"xl/workbook.xml":            tooManySheets.String(),
		"xl/_rels/workbook.xml.rels": `<Relationships/>`,
	})
	_, err = parser.Parse(context.Background(), ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", input)
	require.ErrorIs(t, err, ErrOfficeContentLimit)

	_, err = parser.Parse(context.Background(), ".doc", "application/msword", []byte("legacy"))
	require.ErrorIs(t, err, ErrUnsupportedFormat)
	_, err = parser.Parse(context.Background(), ".xls", "application/vnd.ms-excel", []byte("legacy"))
	require.ErrorIs(t, err, ErrUnsupportedFormat)

	archiveBomb := officeTestArchive(t, map[string]string{
		"[Content_Types].xml": strings.Repeat("a", maxOfficeEntryBytes+1),
		"word/document.xml":   `<w:document/>`,
	})
	_, err = parser.Parse(context.Background(), ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", archiveBomb)
	require.ErrorIs(t, err, ErrOfficeArchiveLimit)
	require.Equal(t, "OFFICE_ARCHIVE_LIMIT", ErrorCode(err))
}

func TestParserExplicitFailures(t *testing.T) {
	parser := NewParser(markdown.NewService(), 4)

	_, err := parser.Parse(context.Background(), ".txt", "text/plain", []byte{0xff})
	require.ErrorIs(t, err, ErrInvalidUTF8)
	require.Equal(t, "INVALID_UTF8", ErrorCode(err))
	_, err = parser.Parse(context.Background(), ".txt", "text/plain", []byte{'a', 0, 'b'})
	require.ErrorIs(t, err, ErrBinaryContent)
	_, err = parser.Parse(context.Background(), ".txt", "text/plain", []byte("12345"))
	require.ErrorIs(t, err, ErrInputTooLarge)
	_, err = parser.Parse(context.Background(), ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", []byte("pptx"))
	require.ErrorIs(t, err, ErrUnsupportedFormat)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = parser.Parse(ctx, ".txt", "text/plain", []byte("ok"))
	require.ErrorIs(t, err, ErrTimeout)
	require.Equal(t, "TIMEOUT", ErrorCode(err))
}

func officeTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}
