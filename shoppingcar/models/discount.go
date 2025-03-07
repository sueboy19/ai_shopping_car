package models

import (
	"time"
)

// 折扣類型
type DiscountType string

const (
	Percentage DiscountType = "PERCENTAGE" // 百分比折扣
	Fixed      DiscountType = "FIXED"      // 固定金額折扣
	Threshold  DiscountType = "THRESHOLD"  // 滿額折扣
	BOGO       DiscountType = "BOGO"       // 買一送一
	MultiItem  DiscountType = "MULTI_ITEM" // 多件折扣
)

// 折扣條件類型
type ConditionType string

const (
	CartTotal       ConditionType = "CART_TOTAL"       // 購物車總額
	MembershipLevel ConditionType = "MEMBERSHIP_LEVEL" // 會員等級
	ProductCategory ConditionType = "PRODUCT_CATEGORY" // 商品類別
	MinQuantity     ConditionType = "MIN_QUANTITY"     // 最低購買數量
)

type DiscountPriority int

const (
	PriorityHigh   DiscountPriority = 1
	PriorityMedium DiscountPriority = 2
	PriorityLow    DiscountPriority = 3
)

type Discount struct {
	ID         int64            `json:"id" gorm:"primaryKey"`
	Name       string           `json:"name" gorm:"size:255"`
	Type       DiscountType     `json:"type" gorm:"size:50"`
	Value      float64          `json:"value" gorm:"type:decimal(10,2)"`
	StartDate  time.Time        `json:"start_date"`
	EndDate    time.Time        `json:"end_date"`
	Priority   DiscountPriority `json:"priority"`
	Stackable  bool             `json:"stackable" gorm:"type:boolean"` // 是否可疊加
	MaxUsage   int              `json:"max_usage"`                     // 最大使用次數
	UsageCount int              `json:"usage_count"`                   // 已使用次數
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`

	Conditions []DiscountCondition `json:"conditions" gorm:"foreignKey:DiscountID"`
	Products   []DiscountProduct   `json:"products" gorm:"foreignKey:DiscountID"`
}
