package models

import (
	"time"
)

type DiscountCondition struct {
	ID         int64         `json:"id" gorm:"primaryKey"`
	DiscountID int64         `json:"discount_id" gorm:"index"`
	Type       ConditionType `json:"type" gorm:"size:50"`   // 使用 discount.go 中定義的 ConditionType
	Value      string        `json:"value" gorm:"size:255"` // 條件值，如: 會員等級、最低消費額等
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
}
