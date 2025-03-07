package models

import (
	"time"
)

type DiscountProduct struct {
	ID         int64     `json:"id" gorm:"primaryKey"`
	DiscountID int64     `json:"discount_id"`
	ProductID  int64     `json:"product_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
