package models

// 订单结构体
type Order struct {
	Material string
	Remark   string
	Width    float64
	Length   float64
}

// 已放置的订单位置
type PlacedOrder struct {
	Order
	X float64
	Y float64
}

// 空闲区域结构体
type FreeRect struct {
	X      float64
	Y      float64
	Width  float64
	Length float64
}

// 布料结构体
type Fabric struct {
	Width      float64
	Length     float64
	Placed     []PlacedOrder
	FreeSpaces []FreeRect
}
