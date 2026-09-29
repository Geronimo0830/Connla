package document

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	maxOfficeArchiveEntries = 2048
	maxOfficeExpandedBytes  = 64 << 20
	maxOfficeEntryBytes     = 32 << 20
	maxOfficeTextBytes      = 16 << 20
	maxDOCXBlocks           = 100000
	maxDOCXTableCells       = 200000
	maxXLSXSheets           = 100
	maxXLSXCells            = 200000
)

type officeArchive struct {
	files map[string]*zip.File
}

func openOfficeArchive(input []byte) (*officeArchive, error) {
	reader, err := zip.NewReader(bytes.NewReader(input), int64(len(input)))
	if err != nil {
		return nil, errors.Wrap(ErrInvalidOffice, err.Error())
	}
	if len(reader.File) == 0 {
		return nil, ErrInvalidOffice
	}
	if len(reader.File) > maxOfficeArchiveEntries {
		return nil, ErrOfficeArchiveLimit
	}
	files := make(map[string]*zip.File, len(reader.File))
	var expanded uint64
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		if strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || strings.HasPrefix(name, "../") {
			return nil, ErrInvalidOffice
		}
		if file.UncompressedSize64 > maxOfficeEntryBytes {
			return nil, ErrOfficeArchiveLimit
		}
		expanded += file.UncompressedSize64
		if expanded > maxOfficeExpandedBytes {
			return nil, ErrOfficeArchiveLimit
		}
		if _, exists := files[name]; exists {
			return nil, ErrInvalidOffice
		}
		files[name] = file
	}
	if files["[Content_Types].xml"] == nil {
		return nil, ErrInvalidOffice
	}
	return &officeArchive{files: files}, nil
}

func (a *officeArchive) read(name string) ([]byte, error) {
	file := a.files[name]
	if file == nil {
		return nil, ErrInvalidOffice
	}
	reader, err := file.Open()
	if err != nil {
		return nil, errors.Wrap(ErrInvalidOffice, err.Error())
	}
	defer reader.Close()
	content, err := io.ReadAll(io.LimitReader(reader, maxOfficeEntryBytes+1))
	if err != nil {
		return nil, errors.Wrap(ErrInvalidOffice, err.Error())
	}
	if len(content) > maxOfficeEntryBytes {
		return nil, ErrOfficeArchiveLimit
	}
	return content, nil
}

type docxBlock struct {
	Type    string          `json:"type"`
	Locator string          `json:"locator"`
	Style   string          `json:"style,omitempty"`
	Text    string          `json:"text,omitempty"`
	Rows    [][]string      `json:"rows,omitempty"`
	Cells   []docxTableCell `json:"cells,omitempty"`
}

type docxTableCell struct {
	Locator string `json:"locator"`
	Row     int    `json:"row"`
	Column  int    `json:"column"`
	Text    string `json:"text"`
}

func (*parser) parseDOCX(ctx context.Context, input []byte) (*Result, error) {
	archive, err := openOfficeArchive(input)
	if err != nil {
		return nil, err
	}
	documentXML, err := archive.read("word/document.xml")
	if err != nil {
		return nil, err
	}
	decoder := xml.NewDecoder(bytes.NewReader(documentXML))
	blocks := make([]docxBlock, 0)
	plain := make([]string, 0)
	tableDepth := 0
	paragraphIndex := 0
	tableIndex := 0
	tableCellCount := 0
	var paragraph strings.Builder
	paragraphStyle := ""
	inParagraph := false
	var tableRows [][]string
	var tableRow []string
	var cellParagraphs []string

	for {
		if err := parserContextError(ctx); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(ErrInvalidOffice, err.Error())
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "tbl":
				tableDepth++
				if tableDepth == 1 {
					tableRows = nil
				}
			case "tr":
				if tableDepth == 1 {
					tableRow = nil
				}
			case "tc":
				if tableDepth == 1 {
					tableCellCount++
					if tableCellCount > maxDOCXTableCells {
						return nil, ErrOfficeContentLimit
					}
					cellParagraphs = nil
				}
			case "p":
				inParagraph = true
				paragraph.Reset()
				paragraphStyle = ""
			case "pStyle":
				if inParagraph {
					paragraphStyle = xmlAttribute(element.Attr, "val")
				}
			case "t":
				if inParagraph {
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						return nil, errors.Wrap(ErrInvalidOffice, err.Error())
					}
					paragraph.WriteString(value)
				}
			case "tab":
				if inParagraph {
					paragraph.WriteByte('\t')
				}
			case "br", "cr":
				if inParagraph {
					paragraph.WriteByte('\n')
				}
			default:
			}
		case xml.EndElement:
			switch element.Name.Local {
			case "p":
				text := strings.TrimSpace(paragraph.String())
				inParagraph = false
				if tableDepth == 0 {
					paragraphIndex++
					blocks = append(blocks, docxBlock{Type: "paragraph", Locator: "paragraph:" + strconv.Itoa(paragraphIndex), Style: paragraphStyle, Text: text})
					if text != "" {
						plain = append(plain, text)
					}
				} else if tableDepth == 1 && text != "" {
					cellParagraphs = append(cellParagraphs, text)
				}
			case "tc":
				if tableDepth == 1 {
					tableRow = append(tableRow, strings.Join(cellParagraphs, "\n"))
				}
			case "tr":
				if tableDepth == 1 {
					tableRows = append(tableRows, tableRow)
				}
			case "tbl":
				if tableDepth == 1 {
					tableIndex++
					locator := "table:" + strconv.Itoa(tableIndex)
					cells := make([]docxTableCell, 0)
					for rowIndex, row := range tableRows {
						for columnIndex, text := range row {
							cells = append(cells, docxTableCell{
								Locator: locator + ":R" + strconv.Itoa(rowIndex+1) + "C" + strconv.Itoa(columnIndex+1),
								Row:     rowIndex + 1, Column: columnIndex + 1, Text: text,
							})
						}
					}
					blocks = append(blocks, docxBlock{Type: "table", Locator: locator, Rows: tableRows, Cells: cells})
					for _, row := range tableRows {
						plain = append(plain, strings.Join(row, "\t"))
					}
				}
				tableDepth--
			default:
			}
		default:
		}
		if len(blocks) > maxDOCXBlocks {
			return nil, ErrOfficeContentLimit
		}
	}
	return officeResult(DOCXParserVersion, plain, struct {
		Version int         `json:"version"`
		Format  string      `json:"format"`
		Blocks  []docxBlock `json:"blocks"`
	}{Version: 1, Format: "docx", Blocks: blocks})
}

type xlsxCell struct {
	Reference string `json:"reference"`
	Value     string `json:"value,omitempty"`
	Formula   string `json:"formula,omitempty"`
	Type      string `json:"type,omitempty"`
}

type xlsxSheet struct {
	Index int        `json:"index"`
	Name  string     `json:"name"`
	Cells []xlsxCell `json:"cells"`
}

type workbookSheet struct {
	Name         string
	Relationship string
}

func (*parser) parseXLSX(ctx context.Context, input []byte) (*Result, error) {
	archive, err := openOfficeArchive(input)
	if err != nil {
		return nil, err
	}
	workbookXML, err := archive.read("xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	relationshipXML, err := archive.read("xl/_rels/workbook.xml.rels")
	if err != nil {
		return nil, err
	}
	workbookSheets, err := parseWorkbookSheets(workbookXML)
	if err != nil || len(workbookSheets) == 0 {
		return nil, ErrInvalidOffice
	}
	if len(workbookSheets) > maxXLSXSheets {
		return nil, ErrOfficeContentLimit
	}
	relationships, err := parseWorkbookRelationships(relationshipXML)
	if err != nil {
		return nil, ErrInvalidOffice
	}
	sharedStrings := []string{}
	if archive.files["xl/sharedStrings.xml"] != nil {
		sharedXML, err := archive.read("xl/sharedStrings.xml")
		if err != nil {
			return nil, err
		}
		sharedStrings, err = parseSharedStrings(sharedXML)
		if err != nil {
			return nil, ErrInvalidOffice
		}
	}

	sheets := make([]xlsxSheet, 0, len(workbookSheets))
	plain := make([]string, 0)
	totalCells := 0
	for index, sheet := range workbookSheets {
		if err := parserContextError(ctx); err != nil {
			return nil, err
		}
		target := relationships[sheet.Relationship]
		if target == "" {
			return nil, ErrInvalidOffice
		}
		worksheetPath, ok := resolveWorkbookTarget(target)
		if !ok {
			return nil, ErrInvalidOffice
		}
		worksheetXML, err := archive.read(worksheetPath)
		if err != nil {
			return nil, err
		}
		cells, err := parseWorksheetCells(ctx, worksheetXML, sharedStrings, maxXLSXCells-totalCells)
		if err != nil {
			return nil, err
		}
		totalCells += len(cells)
		if totalCells > maxXLSXCells {
			return nil, ErrOfficeContentLimit
		}
		sheets = append(sheets, xlsxSheet{Index: index + 1, Name: sheet.Name, Cells: cells})
		plain = append(plain, "["+sheet.Name+"]")
		for _, cell := range cells {
			value := cell.Value
			if cell.Formula != "" {
				value = "=" + cell.Formula
				if cell.Value != "" {
					value += " [cached: " + cell.Value + "]"
				}
			}
			plain = append(plain, cell.Reference+": "+value)
		}
	}
	return officeResult(XLSXParserVersion, plain, struct {
		Version int         `json:"version"`
		Format  string      `json:"format"`
		Sheets  []xlsxSheet `json:"sheets"`
	}{Version: 1, Format: "xlsx", Sheets: sheets})
}

func parseWorkbookSheets(content []byte) ([]workbookSheet, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var sheets []workbookSheet
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return sheets, nil
		}
		if err != nil {
			return nil, err
		}
		if element, ok := token.(xml.StartElement); ok && element.Name.Local == "sheet" {
			sheets = append(sheets, workbookSheet{Name: xmlAttribute(element.Attr, "name"), Relationship: xmlAttribute(element.Attr, "id")})
		}
	}
}

func parseWorkbookRelationships(content []byte) (map[string]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	relationships := map[string]string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return relationships, nil
		}
		if err != nil {
			return nil, err
		}
		if element, ok := token.(xml.StartElement); ok && element.Name.Local == "Relationship" {
			relationships[xmlAttribute(element.Attr, "Id")] = xmlAttribute(element.Attr, "Target")
		}
	}
}

func parseSharedStrings(content []byte) ([]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var values []string
	var current strings.Builder
	inString := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return values, nil
		}
		if err != nil {
			return nil, err
		}
		switch element := token.(type) {
		case xml.StartElement:
			if element.Name.Local == "si" {
				current.Reset()
				inString = true
			} else if element.Name.Local == "t" && inString {
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return nil, err
				}
				current.WriteString(value)
			}
		case xml.EndElement:
			if element.Name.Local == "si" {
				values = append(values, current.String())
				inString = false
			}
		default:
		}
	}
}

func parseWorksheetCells(ctx context.Context, content []byte, sharedStrings []string, remaining int) ([]xlsxCell, error) {
	if remaining < 0 {
		return nil, ErrOfficeContentLimit
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	var cells []xlsxCell
	var current xlsxCell
	inCell := false
	for {
		if err := parserContextError(ctx); err != nil {
			return nil, err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			return cells, nil
		}
		if err != nil {
			return nil, errors.Wrap(ErrInvalidOffice, err.Error())
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch element.Name.Local {
			case "c":
				current = xlsxCell{Reference: xmlAttribute(element.Attr, "r"), Type: xmlAttribute(element.Attr, "t")}
				inCell = true
			case "f":
				if inCell {
					if err := decoder.DecodeElement(&current.Formula, &element); err != nil {
						return nil, errors.Wrap(ErrInvalidOffice, err.Error())
					}
				}
			case "v":
				if inCell {
					if err := decoder.DecodeElement(&current.Value, &element); err != nil {
						return nil, errors.Wrap(ErrInvalidOffice, err.Error())
					}
				}
			case "t":
				if inCell && current.Type == "inlineStr" {
					var value string
					if err := decoder.DecodeElement(&value, &element); err != nil {
						return nil, errors.Wrap(ErrInvalidOffice, err.Error())
					}
					current.Value += value
				}
			default:
			}
		case xml.EndElement:
			if element.Name.Local == "c" {
				inCell = false
				if current.Reference == "" {
					return nil, ErrInvalidOffice
				}
				if current.Type == "s" {
					index, err := strconv.Atoi(current.Value)
					if err != nil || index < 0 || index >= len(sharedStrings) {
						return nil, ErrInvalidOffice
					}
					current.Value = sharedStrings[index]
				}
				if current.Value != "" || current.Formula != "" {
					cells = append(cells, current)
					if len(cells) > remaining {
						return nil, ErrOfficeContentLimit
					}
				}
			}
		default:
		}
	}
}

func resolveWorkbookTarget(target string) (string, bool) {
	target = strings.ReplaceAll(target, "\\", "/")
	if strings.HasPrefix(target, "/") {
		target = strings.TrimPrefix(target, "/")
	} else {
		target = path.Join("xl", target)
	}
	clean := path.Clean(target)
	return clean, clean != "." && !strings.HasPrefix(clean, "../")
}

func xmlAttribute(attributes []xml.Attr, localName string) string {
	for _, attribute := range attributes {
		if attribute.Name.Local == localName {
			return attribute.Value
		}
	}
	return ""
}

func officeResult(extractor string, plainParts []string, structuredValue any) (*Result, error) {
	plainText := strings.TrimSpace(strings.Join(plainParts, "\n"))
	if len(plainText) > maxOfficeTextBytes {
		return nil, ErrOfficeContentLimit
	}
	structured, err := json.Marshal(structuredValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to encode Office parser metadata")
	}
	if len(structured) > maxOfficeTextBytes {
		return nil, ErrOfficeContentLimit
	}
	hash := sha256.Sum256([]byte(plainText))
	return &Result{
		PlainText:      plainText,
		StructuredJSON: string(structured),
		ContentHash:    hex.EncodeToString(hash[:]),
		Extractor:      extractor,
		Warnings:       []string{},
	}, nil
}
