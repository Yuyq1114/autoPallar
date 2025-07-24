package utils

import (
	"autoPallar/models"
	"fmt"
	"github.com/xuri/excelize/v2"
	"regexp"
	"strconv"
	"strings"
)

func extractSize(text string) (float64, float64, bool) {
	// 支持 35*120、35-124.5、35×120、34 x 120cm 等格式
	re := regexp.MustCompile(`(\d+(.\d+)?)[×x*-](\d+(.\d+)?)`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 4 {
		width, _ := strconv.ParseFloat(match[1], 64)
		length, _ := strconv.ParseFloat(match[3], 64)
		return width, length, true
	}
	return 0, 0, false
}

func ReadExcel(filePath string, column string) ([]models.Order, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, err
	}

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}

	// 找出指定列位置
	colIndex := -1
	header := rows[0]
	for i, val := range header {
		if val == column {
			colIndex = i
			break
		}
	}
	if colIndex == -1 {
		return nil, fmt.Errorf("找不到备注列：%s", column)
	}

	var result []models.Order
	for _, row := range rows[1:] {
		if len(row) <= colIndex {
			continue
		}
		text := row[colIndex]
		marterial := ""
		if strings.Contains(text, "PVC") || strings.Contains(text, "pvc") {
			marterial = "PVC"
		}
		if strings.Contains(text, "皮革") {
			marterial = "皮革"
		}
		if strings.Contains(text, "绒") {
			marterial = "绒"
		}

		if w, l, ok := extractSize(text); ok {
			result = append(result, models.Order{Material: marterial, Remark: text, Width: w, Length: l})
		}
	}
	return result, nil
}

//func main() {
//	orders, err := readExcel(`C:\Users\31178\Desktop\zhubu.xlsx`, "卖家备注")
//	if err != nil {
//		panic(err)
//	}
//
//	for _, o := range orders {
//		fmt.Printf("✔ %s => 宽: %.1fcm, 长: %.1fcm\n", o.Remark, o.Width, o.Length)
//	}
//}
