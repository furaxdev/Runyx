package convert

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type xlWorkbook struct {
	Sheets []xlSheetRef `xml:"sheets>sheet"`
}

type xlSheetRef struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"id,attr"` // r:id, Go keeps only the local name "id"
}

type xlSST struct {
	Items []xlSI `xml:"si"`
}

type xlSI struct {
	T  string  `xml:"t"`
	Rs []xlRun `xml:"r"`
}

type xlRun struct {
	T string `xml:"t"`
}

func (si xlSI) text() string {
	if si.T != "" {
		return si.T
	}
	var b strings.Builder
	for _, r := range si.Rs {
		b.WriteString(r.T)
	}
	return b.String()
}

type xlSheetData struct {
	Rows []xlRow `xml:"sheetData>row"`
}

type xlRow struct {
	Cells []xlC `xml:"c"`
}

type xlC struct {
	R string `xml:"r,attr"`
	T string `xml:"t,attr"` // "s" = shared string, "" = number/inline
	V string `xml:"v"`
}

// convertXLSX renders every sheet as a Markdown table. Workbook
// relationships (sheetN.xml <-> sheet name) are approximated by file
// order, which matches Excel's own default save order in the vast
// majority of real files; exact r:id resolution via
// xl/_rels/workbook.xml.rels is a known gap if a file was hand-edited.
func convertXLSX(path, name string) (Result, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("open xlsx %s: %w", name, err)
	}
	defer r.Close()

	var wb xlWorkbook
	if data, err := readZipMember(r, "xl/workbook.xml"); err == nil {
		xml.Unmarshal(data, &wb)
	}

	var sst xlSST
	if data, err := readZipMember(r, "xl/sharedStrings.xml"); err == nil {
		xml.Unmarshal(data, &sst)
	}

	var sheetFiles []string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetFiles = append(sheetFiles, f.Name)
		}
	}

	var b strings.Builder
	var warnings []string
	for i, sf := range sheetFiles {
		data, err := readZipMember(r, sf)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		var sheet xlSheetData
		if err := xml.Unmarshal(data, &sheet); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", sf, err))
			continue
		}
		title := sf
		if i < len(wb.Sheets) {
			title = wb.Sheets[i].Name
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		for ri, row := range sheet.Rows {
			cells := make([]string, len(row.Cells))
			for ci, c := range row.Cells {
				cells[ci] = cellValue(c, sst)
			}
			fmt.Fprintf(&b, "| %s |\n", strings.Join(cells, " | "))
			if ri == 0 {
				sep := make([]string, len(cells))
				for j := range sep {
					sep[j] = "---"
				}
				fmt.Fprintf(&b, "| %s |\n", strings.Join(sep, " | "))
			}
		}
		b.WriteString("\n")
	}

	return Result{FileName: name, Kind: "xlsx", Text: strings.TrimSpace(b.String()), Warnings: warnings}, nil
}

func cellValue(c xlC, sst xlSST) string {
	if c.T == "s" {
		idx, err := strconv.Atoi(c.V)
		if err == nil && idx >= 0 && idx < len(sst.Items) {
			return sst.Items[idx].text()
		}
	}
	return c.V
}
