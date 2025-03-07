package services

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"shopping_cart/models"

	"gorm.io/gorm"
)

type DiscountService struct {
	db *gorm.DB
}

func NewDiscountService(db *gorm.DB) *DiscountService {
	return &DiscountService{db: db}
}

func (s *DiscountService) CreateDiscount(ctx context.Context, discount *models.Discount) error {
	if discount.StartDate.After(discount.EndDate) {
		return errors.New("start date cannot be after end date")
	}

	discount.CreatedAt = time.Now()
	discount.UpdatedAt = time.Now()

	return s.db.WithContext(ctx).Create(discount).Error
}

func (s *DiscountService) UpdateDiscount(ctx context.Context, id int64, discount *models.Discount) error {
	existing := &models.Discount{}
	if err := s.db.WithContext(ctx).First(existing, id).Error; err != nil {
		return err
	}

	if discount.StartDate.After(discount.EndDate) {
		return errors.New("start date cannot be after end date")
	}

	discount.UpdatedAt = time.Now()
	return s.db.WithContext(ctx).Model(existing).Updates(discount).Error
}

func (s *DiscountService) DeleteDiscount(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&models.Discount{}, id).Error
}

func (s *DiscountService) GetAvailableDiscounts(ctx context.Context, userID int64, cartTotal float64, productIDs []int64) ([]models.Discount, error) {
	var discounts []models.Discount
	now := time.Now()

	// 輸出調試信息以檢查SQL查詢
	log.Printf("查詢折扣，用戶ID: %d, 購物車總額: %f, 商品IDs: %v", userID, cartTotal, productIDs)

	// 獲取所有有效折扣
	query := s.db.WithContext(ctx).Debug(). // 添加 Debug() 以記錄 SQL 查詢
						Where("start_date <= ? AND end_date >= ?", now, now)

	// 根據用戶條件過濾
	if userID != 0 {
		query = query.Joins("JOIN discount_conditions ON discount_conditions.discount_id = discounts.id").
			Where("discount_conditions.type = ? AND discount_conditions.value = ?", models.MembershipLevel, "GOLD")
	}

	// 根據購物車總金額過濾
	if cartTotal > 0 {
		// 使用原始SQL進行JOIN和條件處理
		query = query.Joins("JOIN discount_conditions ON discount_conditions.discount_id = discounts.id").
			Where("(discount_conditions.type = ? AND CAST(discount_conditions.value AS DECIMAL) <= ?)",
				models.CartTotal, cartTotal)
	}

	// 根據商品ID過濾
	if len(productIDs) > 0 {
		query = query.Joins("JOIN discount_products ON discount_products.discount_id = discounts.id").
			Where("discount_products.product_id IN ?", productIDs)
	}

	// 執行查詢
	if err := query.Find(&discounts).Error; err != nil {
		return nil, err
	}
	log.Printf("查詢到 %d 個有效折扣", len(discounts))

	// 過濾已達最大使用次數的折扣
	filteredDiscounts := make([]models.Discount, 0)
	for _, discount := range discounts {
		if discount.MaxUsage == 0 || discount.UsageCount < discount.MaxUsage {
			filteredDiscounts = append(filteredDiscounts, discount)
		} else {
			continue
		}
	}

	// 根據優先級和可疊加性排序 - 修正排序邏輯確保高優先級在前
	sort.Slice(filteredDiscounts, func(i, j int) bool {
		// 首先按優先級排序（數字越小優先級越高）
		if filteredDiscounts[i].Priority != filteredDiscounts[j].Priority {
			return filteredDiscounts[i].Priority < filteredDiscounts[j].Priority
		}

		// 相同優先級時，不可疊加的折扣優先
		return !filteredDiscounts[i].Stackable
	})

	// 移除了更新使用次數的部分，只在結帳時才更新使用次數

	return filteredDiscounts, nil
}

// 新增一個方法，用於結帳時更新折扣使用次數
func (s *DiscountService) UpdateDiscountUsage(ctx context.Context, discountIDs []int64) error {
	if len(discountIDs) == 0 {
		return nil
	}

	// 使用交易確保更新的原子性
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var discounts []models.Discount
		if err := tx.Find(&discounts, "id IN ?", discountIDs).Error; err != nil {
			return err
		}

		for _, discount := range discounts {
			// 只更新有使用次數限制的折扣
			if discount.MaxUsage > 0 {
				discount.UsageCount++
				if err := tx.Model(&discount).Update("usage_count", discount.UsageCount).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}
