// main.go
package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"log"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ====== 数据结构 ======

type Order struct {
	Material string  `json:"material"`
	Remark   string  `json:"remark"`
	Width    float64 `json:"width"`
	Length   float64 `json:"length"`
}

type PlacedOrder struct {
	Order
	FabricIndex int     `json:"fabricIndex"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
}

type FabricStats struct {
	FabricIndex  int           `json:"fabricIndex"`
	Material     string        `json:"material"`
	FabricWidth  float64       `json:"fabricWidth"`
	UsedArea     float64       `json:"usedArea"`
	TotalArea    float64       `json:"totalArea"`
	WasteArea    float64       `json:"wasteArea"`
	UsageRate    float64       `json:"usageRate"`
	PlacedOrders []PlacedOrder `json:"placedOrders"`
}

type GlobalStats struct {
	TotalUsedArea  float64 `json:"totalUsedArea"`
	TotalArea      float64 `json:"totalArea"`
	TotalWasteArea float64 `json:"totalWasteArea"`
	OverallRate    float64 `json:"overallRate"`
}

type LayoutResult struct {
	Materials map[string][]FabricStats `json:"materials"`
	Summary   GlobalStats              `json:"summary"`
}

// ========== MaxRects 算法 ==========

type Rect struct {
	X, Y          float64
	Width, Height float64
}

type MaxRectsBin struct {
	Width, Height float64
	FreeRects     []Rect
	Placed        []PlacedOrder
	Gap           float64
	FabricIndex   int
}

func NewMaxRectsBin(width, height float64, gap float64, fabricIndex int) *MaxRectsBin {
	return &MaxRectsBin{
		Width:       width,
		Height:      height,
		Gap:         gap,
		FabricIndex: fabricIndex,
		FreeRects:   []Rect{{0, 0, width, height}},
		Placed:      []PlacedOrder{},
	}
}

func (bin *MaxRectsBin) Insert(order Order) (bool, PlacedOrder) {
	bestScore := math.MaxFloat64
	var bestNode Rect
	var freeIndex int

	for i, free := range bin.FreeRects {
		if order.Width+bin.Gap <= free.Width && order.Length+bin.Gap <= free.Height {
			score := free.Width*free.Height - order.Width*order.Length
			if score < bestScore {
				bestScore = score
				bestNode = Rect{free.X, free.Y, order.Width, order.Length}
				freeIndex = i
			}
		}
	}

	if bestScore == math.MaxFloat64 {
		return false, PlacedOrder{}
	}

	// 拆分空白区域
	used := bin.FreeRects[freeIndex]
	bin.FreeRects = append(bin.FreeRects[:freeIndex], bin.FreeRects[freeIndex+1:]...)

	right := Rect{
		X:      used.X + order.Width + bin.Gap,
		Y:      used.Y,
		Width:  used.Width - order.Width - bin.Gap,
		Height: order.Length,
	}
	if right.Width > 0 && right.Height > 0 {
		bin.FreeRects = append(bin.FreeRects, right)
	}
	bottom := Rect{
		X:      used.X,
		Y:      used.Y + order.Length + bin.Gap,
		Width:  used.Width,
		Height: used.Height - order.Length - bin.Gap,
	}
	if bottom.Width > 0 && bottom.Height > 0 {
		bin.FreeRects = append(bin.FreeRects, bottom)
	}

	placed := PlacedOrder{
		Order:       order,
		FabricIndex: bin.FabricIndex,
		X:           bestNode.X,
		Y:           bestNode.Y,
	}
	bin.Placed = append(bin.Placed, placed)
	return true, placed
}

// ========== 排布主函数 ==========

type WidthMaterialKey struct {
	Material string
	Width    float64
}

func ArrangeOrdersWithMaxRects(orders []Order, fabricWidths []float64, fabricLength, gap float64) LayoutResult {
	grouped := make(map[WidthMaterialKey][]Order)

	for _, order := range orders {
		if order.Length > fabricLength {
			continue
		}
		for _, fw := range fabricWidths {
			if order.Width <= fw {
				key := WidthMaterialKey{order.Material, fw}
				grouped[key] = append(grouped[key], order)
				break
			}
		}
	}

	allStats := make(map[string][]FabricStats)
	var totalUsed, totalArea float64

	for key, group := range grouped {
		sort.Slice(group, func(i, j int) bool {
			return group[i].Length > group[j].Length
		})
		var bins []*MaxRectsBin
		bins = append(bins, NewMaxRectsBin(key.Width, fabricLength, gap, 1))

		for _, order := range group {
			placed := false
			for _, bin := range bins {
				ok, _ := bin.Insert(order)
				if ok {
					placed = true
					break
				}
			}
			if !placed {
				newIndex := len(bins) + 1
				newBin := NewMaxRectsBin(key.Width, fabricLength, gap, newIndex)
				_, _ = newBin.Insert(order)
				bins = append(bins, newBin)
			}
		}

		for _, bin := range bins {
			var used, maxY float64
			for _, p := range bin.Placed {
				used += p.Width * p.Length
				if y := p.Y + p.Length; y > maxY {
					maxY = y
				}
			}
			actualArea := bin.Width * maxY
			allStats[key.Material] = append(allStats[key.Material], FabricStats{
				FabricIndex:  bin.FabricIndex,
				Material:     key.Material,
				FabricWidth:  key.Width,
				UsedArea:     used,
				TotalArea:    actualArea,
				WasteArea:    actualArea - used,
				UsageRate:    used / actualArea,
				PlacedOrders: bin.Placed,
			})
			totalUsed += used
			totalArea += actualArea
		}
	}

	summary := GlobalStats{
		TotalUsedArea:  totalUsed,
		TotalArea:      totalArea,
		TotalWasteArea: totalArea - totalUsed,
		OverallRate:    totalUsed / totalArea,
	}

	return LayoutResult{
		Materials: allStats,
		Summary:   summary,
	}
}

// ========== 读取 Excel 与接口 ==========

func extractSize(text string) (float64, float64, bool) {
	re := regexp.MustCompile(`(\d+(\.\d+)?)[×x*-](\d+(\.\d+)?)`)
	match := re.FindStringSubmatch(text)
	if len(match) >= 4 {
		width, _ := strconv.ParseFloat(match[1], 64)
		length, _ := strconv.ParseFloat(match[3], 64)
		return width, length, true
	}
	return 0, 0, false
}

func ReadExcel(filePath string, column string) ([]Order, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	colIndex := -1
	for i, val := range rows[0] {
		if val == column {
			colIndex = i
			break
		}
	}
	if colIndex == -1 {
		return nil, fmt.Errorf("找不到备注列：%s", column)
	}

	var result []Order
	for _, row := range rows[1:] {
		if len(row) <= colIndex {
			continue
		}
		text := row[colIndex]
		material := ""
		switch {
		case strings.Contains(strings.ToLower(text), "pvc"):
			material = "PVC"
		case strings.Contains(text, "皮"):
			material = "皮革"
		case strings.Contains(text, "绒"):
			material = "绒"
		default:
			material = "未知"
		}
		if material != "PVC" {
			continue
		}
		if w, l, ok := extractSize(text); ok {
			if w > l {
				w, l = l, w
			}
			result = append(result, Order{
				Material: material,
				Remark:   text,
				Width:    w,
				Length:   l,
			})
		}
	}
	return result, nil
}

func handleLayout(c *gin.Context) {
	orders, err := ReadExcel(`C:\Users\31178\Desktop\zhubu.xlsx`, "卖家备注")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	//var orders []Order
	//order := Order{
	//	Material: "PVC",
	//	Remark:   "<UNK>",
	//	Width:    155,
	//	Length:   2800,
	//}
	//orders = append(orders, order)
	//order = Order{
	//	Material: "PVC",
	//	Remark:   "<UNK>",
	//	Width:    135,
	//	Length:   2800,
	//}
	//orders = append(orders, order)
	//order = Order{
	//	Material: "PVC",
	//	Remark:   "<UNK>",
	//	Width:    175,
	//	Length:   2800,
	//}
	//orders = append(orders, order)

	result := FindBestLayout(orders, []float64{140, 160, 180}, 3000, 0.4)

	//result := ArrangeOrdersWithMaxRects(orders, []float64{140, 160, 180}, 3000, 0.4)
	//result := ArrangeOrdersWithMaxRects(orders, []float64{160, 180, 140}, 3000, 0.4)
	//result := ArrangeOrdersWithMaxRects(orders, []float64{180, 140, 160}, 3000, 0.4)
	c.JSON(http.StatusOK, result)
}
func permuteFabricWidths(input []float64) [][]float64 {
	var res [][]float64
	var helper func([]float64, int)
	helper = func(arr []float64, n int) {
		if n == 1 {
			cp := make([]float64, len(arr))
			copy(cp, arr)
			res = append(res, cp)
			return
		}
		for i := 0; i < n; i++ {
			arr[i], arr[n-1] = arr[n-1], arr[i]
			helper(arr, n-1)
			arr[i], arr[n-1] = arr[n-1], arr[i]
		}
	}
	helper(input, len(input))
	return res
}
func FindBestLayout(orders []Order, fabricWidths []float64, fabricLength, gap float64) LayoutResult {
	permutations := permuteFabricWidths(fabricWidths)
	var best LayoutResult
	minWaste := math.MaxFloat64

	for _, order := range permutations {
		result := ArrangeOrdersWithMaxRects(orders, order, fabricLength, gap)
		if result.Summary.TotalWasteArea < minWaste {
			minWaste = result.Summary.TotalWasteArea
			best = result
		}
	}
	return best
}

func main() {
	r := gin.Default()
	r.POST("/layout", handleLayout)
	r.Static("/", "./static")
	log.Println("Server running at http://localhost:8080")
	r.Run(":8080")
}
