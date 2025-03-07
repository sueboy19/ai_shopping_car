package services

import (
	"context"
	"sort"
	"testing"
	"time"

	"shopping_cart/models"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// 自動遷移數據庫表
	if err := db.AutoMigrate(
		&models.Discount{},
		&models.DiscountCondition{},
		&models.DiscountProduct{},
	); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func TestCreateDiscount(t *testing.T) {
	db := setupTestDB(t)
	service := NewDiscountService(db)

	discount := &models.Discount{
		Name:      "Test Discount",
		Type:      models.Percentage,
		Value:     10.0,
		StartDate: time.Now(),
		EndDate:   time.Now().Add(24 * time.Hour),
		Priority:  1,
	}

	err := service.CreateDiscount(context.Background(), discount)
	assert.NoError(t, err)
	assert.NotZero(t, discount.ID)

	var count int64
	db.Model(&models.Discount{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestGetAvailableDiscounts(t *testing.T) {
	db := setupTestDB(t)
	service := NewDiscountService(db)

	// 創建多個測試折扣
	discounts := []*models.Discount{
		{
			Name:      "High Priority Discount",
			Type:      models.Percentage,
			Value:     10.0,
			StartDate: time.Now(),
			EndDate:   time.Now().Add(24 * time.Hour),
			Priority:  1,
			Stackable: false,
			MaxUsage:  100,
		},
		{
			Name:      "Medium Priority Discount",
			Type:      models.Fixed,
			Value:     5.0,
			StartDate: time.Now(),
			EndDate:   time.Now().Add(24 * time.Hour),
			Priority:  2,
			Stackable: true,
			MaxUsage:  50,
		},
		{
			Name:      "Low Priority Discount",
			Type:      models.Threshold,
			Value:     20.0,
			StartDate: time.Now(),
			EndDate:   time.Now().Add(24 * time.Hour),
			Priority:  3,
			Stackable: true,
			MaxUsage:  0, // 無限制
		},
	}

	// 創建折扣並添加條件
	for _, discount := range discounts {
		service.CreateDiscount(context.Background(), discount)
		condition := &models.DiscountCondition{
			DiscountID: discount.ID,
			Type:       models.CartTotal,
			Value:      "100",
		}
		db.Create(condition)
	}

	// 測試獲取可用折扣
	availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 150, []int64{})
	assert.NoError(t, err)
	assert.Len(t, availableDiscounts, 3)

	// 驗證優先級排序
	assert.Equal(t, "High Priority Discount", availableDiscounts[0].Name)
	assert.Equal(t, "Medium Priority Discount", availableDiscounts[1].Name)
	assert.Equal(t, "Low Priority Discount", availableDiscounts[2].Name)

	// 將測試最大使用次數的部分改為測試 UpdateDiscountUsage 方法
	for i := 0; i < 100; i++ {
		// 更新高優先級折扣使用次數
		err = service.UpdateDiscountUsage(context.Background(), []int64{discounts[0].ID})
		assert.NoError(t, err)
	}

	// 更新中等優先級折扣使用次數
	for i := 0; i < 50; i++ {
		err = service.UpdateDiscountUsage(context.Background(), []int64{discounts[1].ID})
		assert.NoError(t, err)
	}

	// 驗證高優先級折扣是否達到最大使用次數
	var highPriorityDiscount models.Discount
	db.First(&highPriorityDiscount, "name = ?", "High Priority Discount")
	assert.Equal(t, 100, highPriorityDiscount.UsageCount)

	// 驗證中等優先級折扣是否達到最大使用次數
	var mediumPriorityDiscount models.Discount
	db.First(&mediumPriorityDiscount, "name = ?", "Medium Priority Discount")
	assert.Equal(t, 50, mediumPriorityDiscount.UsageCount)

	// 再次獲取可用折扣，此時高優先級和中等優先級折扣應已達使用上限
	availableDiscounts, err = service.GetAvailableDiscounts(context.Background(), 0, 150, []int64{})
	assert.NoError(t, err)
	assert.Len(t, availableDiscounts, 1) // 僅剩 Low Priority 折扣可用
	assert.Equal(t, "Low Priority Discount", availableDiscounts[0].Name)
}

// 新增一個專門測試 UpdateDiscountUsage 方法的測試函數
func TestUpdateDiscountUsage(t *testing.T) {
	db := setupTestDB(t)
	service := NewDiscountService(db)

	now := time.Now()
	// 創建測試折扣，確保時間範圍正確
	discount := &models.Discount{
		Name:      "Usage Test Discount",
		Type:      models.Percentage,
		Value:     10.0,
		StartDate: now.Add(-1 * time.Hour), // 確保開始時間在當前時間之前
		EndDate:   now.Add(24 * time.Hour), // 確保結束時間在當前時間之後
		Priority:  1,
		MaxUsage:  5,
		Stackable: true,
	}

	err := service.CreateDiscount(context.Background(), discount)
	assert.NoError(t, err)
	assert.NotZero(t, discount.ID)

	// 使用可以確保小於等於比較成功的數字字串
	cartTotal := 100.0
	// 新增一個條件，確保條件值小於我們要測試的購物車總額
	condition := &models.DiscountCondition{
		DiscountID: discount.ID,
		Type:       models.CartTotal,
		Value:      "50", // 這裡設為 50，確保 100 > 50
	}
	err = db.Create(condition).Error
	assert.NoError(t, err)

	// 檢查資料庫中是否確實有這筆折扣資料
	var count int64
	err = db.Model(&models.Discount{}).Where("id = ?", discount.ID).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 檢查資料庫中是否確實有這筆條件資料
	count = 0
	err = db.Model(&models.DiscountCondition{}).Where("discount_id = ?", discount.ID).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 直接執行SQL查詢，驗證條件是否正確
	var rawConditions []struct {
		DiscountID int64
		Type       string
		Value      string
	}
	db.Raw("SELECT discount_id, type, value FROM discount_conditions WHERE discount_id = ?", discount.ID).Scan(&rawConditions)
	t.Logf("Raw condition data: %+v", rawConditions)

	// 輸出實際的SQL查詢結果，用於調試
	var discounts []models.Discount
	err = db.Where("start_date <= ? AND end_date >= ?", now, now).Find(&discounts).Error
	assert.NoError(t, err)
	t.Logf("Found %d discounts in date range", len(discounts))

	// 直接用SQL測試完整的查詢條件
	var matchingDiscounts []models.Discount
	err = db.Raw(`
		SELECT d.* FROM discounts d
		JOIN discount_conditions dc ON dc.discount_id = d.id
		WHERE d.start_date <= ? AND d.end_date >= ?
		AND dc.type = ? AND CAST(dc.value AS DECIMAL) <= ?
	`, now, now, models.CartTotal, cartTotal).Scan(&matchingDiscounts).Error
	assert.NoError(t, err)
	t.Logf("Direct SQL found %d matching discounts", len(matchingDiscounts))

	// 修改為明確的查詢條件
	availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, cartTotal, []int64{})
	assert.NoError(t, err)
	assert.Len(t, availableDiscounts, 1, "Should find one available discount")

	// 更新使用次數
	for i := 0; i < 3; i++ {
		err = service.UpdateDiscountUsage(context.Background(), []int64{discount.ID})
		assert.NoError(t, err)
	}

	// 驗證使用次數
	var updatedDiscount models.Discount
	db.First(&updatedDiscount, discount.ID)
	assert.Equal(t, 3, updatedDiscount.UsageCount)

	// 確認折扣仍然可用
	availableDiscounts, err = service.GetAvailableDiscounts(context.Background(), 0, 100.0, []int64{})
	assert.NoError(t, err)
	assert.Len(t, availableDiscounts, 1, "Discount should still be available")

	// 繼續更新直到達到上限
	for i := 0; i < 2; i++ {
		err = service.UpdateDiscountUsage(context.Background(), []int64{discount.ID})
		assert.NoError(t, err)
	}

	// 再次驗證使用次數
	db.First(&updatedDiscount, discount.ID)
	assert.Equal(t, 5, updatedDiscount.UsageCount)

	// 驗證達到使用上限的折扣不再出現在可用折扣中
	availableDiscounts, err = service.GetAvailableDiscounts(context.Background(), 0, 100.0, []int64{})
	assert.NoError(t, err)
	assert.Len(t, availableDiscounts, 0, "No discounts should be available when usage limit is reached")
}

// 測試各種不同類型的折扣
func TestDifferentDiscountTypes(t *testing.T) {
	db := setupTestDB(t)
	service := NewDiscountService(db)
	now := time.Now()

	// 創建不同類型的折扣
	discounts := []*models.Discount{
		{
			// 1. 百分比折扣 (例如: 9折)
			Name:      "Percentage Discount",
			Type:      models.Percentage,
			Value:     10.0, // 10% 折扣
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  1,
			MaxUsage:  100,
		},
		{
			// 2. 固定金額折扣 (例如: 減$50)
			Name:      "Fixed Amount Discount",
			Type:      models.Fixed,
			Value:     50.0, // 減$50
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  2,
			MaxUsage:  100,
		},
		{
			// 3. 滿額折扣 (例如: 滿$1000減$100)
			Name:      "Threshold Discount",
			Type:      models.Threshold,
			Value:     100.0, // 減$100
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  3,
			MaxUsage:  100,
		},
		{
			// 4. 買一送一
			Name:      "Buy One Get One Free",
			Type:      models.BOGO,
			Value:     1.0, // 買1送1
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  4,
			MaxUsage:  100,
		},
		{
			// 5. 多件折扣 (例如: 第二件半價)
			Name:      "Multi-Item Discount",
			Type:      models.MultiItem,
			Value:     50.0, // 第二件5折
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  5,
			MaxUsage:  100,
		},
	}

	// 創建折扣及其對應條件
	for i, discount := range discounts {
		err := service.CreateDiscount(context.Background(), discount)
		assert.NoError(t, err)

		// 為不同類型的折扣添加不同的條件
		switch discount.Type {
		case models.Percentage, models.Fixed:
			// 百分比和固定金額折扣使用購物車總額條件
			condition := &models.DiscountCondition{
				DiscountID: discount.ID,
				Type:       models.CartTotal,
				Value:      "100", // 購物車總額需達到 100
			}
			db.Create(condition)

		case models.Threshold:
			// 滿額折扣使用特定的購物車總額條件
			condition := &models.DiscountCondition{
				DiscountID: discount.ID,
				Type:       models.CartTotal,
				Value:      "1000", // 需滿 1000 才能享受折扣
			}
			db.Create(condition)

		case models.BOGO, models.MultiItem:
			// 買一送一和多件折扣需要指定特定商品
			productID := int64(i + 1) // 模擬商品ID
			discountProduct := &models.DiscountProduct{
				DiscountID: discount.ID,
				ProductID:  productID,
			}
			db.Create(discountProduct)

			// 設定最小購買數量條件
			condition := &models.DiscountCondition{
				DiscountID: discount.ID,
				Type:       models.MinQuantity,
				Value:      "2", // 最少需要購買2件
			}
			db.Create(condition)
		}
	}

	// 1. 測試百分比折扣
	t.Run("Percentage Discount", func(t *testing.T) {
		// 模擬購物車總額為 200
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)

		// 找到百分比折扣
		var percentageDiscount *models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Name == "Percentage Discount" {
				percentageDiscount = &availableDiscounts[i]
				break
			}
		}

		assert.NotNil(t, percentageDiscount, "應該找到百分比折扣")
		assert.Equal(t, models.Percentage, percentageDiscount.Type)

		// 使用從API獲取的實際折扣值計算折扣金額
		totalAmount := 200.0
		var discountedAmount float64

		// 根據折扣類型計算實際折扣金額
		if percentageDiscount.Type == models.Percentage {
			discountedAmount = totalAmount * (1 - percentageDiscount.Value/100)
		}

		expectedDiscountedAmount := 200.0 * (1 - 10.0/100) // 200 * 0.9 = 180
		assert.Equal(t, expectedDiscountedAmount, discountedAmount)
	})

	// 2. 測試固定金額折扣
	t.Run("Fixed Amount Discount", func(t *testing.T) {
		// 模擬購物車總額為 200
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)

		// 找到固定金額折扣
		var fixedDiscount *models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Name == "Fixed Amount Discount" {
				fixedDiscount = &availableDiscounts[i]
				break
			}
		}

		assert.NotNil(t, fixedDiscount, "應該找到固定金額折扣")
		assert.Equal(t, models.Fixed, fixedDiscount.Type)

		// 使用從API獲取的實際折扣值計算折扣金額
		totalAmount := 200.0
		var discountedAmount float64

		// 根據折扣類型計算實際折扣金額
		if fixedDiscount.Type == models.Fixed {
			discountedAmount = totalAmount - fixedDiscount.Value
		}

		assert.Equal(t, 150.0, discountedAmount)
	})

	// 3. 測試滿額折扣
	t.Run("Threshold Discount", func(t *testing.T) {
		// 購物車總額未達到門檻
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 500, []int64{})
		assert.NoError(t, err)

		// 檢查未達門檻時是否找不到滿額折扣
		thresholdFound := false
		for _, d := range availableDiscounts {
			if d.Name == "Threshold Discount" {
				thresholdFound = true
				break
			}
		}
		assert.False(t, thresholdFound, "未達到門檻時不應該找到滿額折扣")

		// 購物車總額達到門檻
		availableDiscounts, err = service.GetAvailableDiscounts(context.Background(), 0, 1200, []int64{})
		assert.NoError(t, err)

		// 找到滿額折扣
		var thresholdDiscount *models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Name == "Threshold Discount" {
				thresholdDiscount = &availableDiscounts[i]
				break
			}
		}

		assert.NotNil(t, thresholdDiscount, "達到門檻時應該找到滿額折扣")
		assert.Equal(t, models.Threshold, thresholdDiscount.Type)

		// 使用從API獲取的實際折扣值計算折扣金額
		totalAmount := 1200.0
		var discountedAmount float64

		// 根據折扣類型計算實際折扣金額
		if thresholdDiscount.Type == models.Threshold {
			discountedAmount = totalAmount - thresholdDiscount.Value
		}

		assert.Equal(t, 1100.0, discountedAmount)
	})

	// 4. 測試買一送一
	t.Run("Buy One Get One Free", func(t *testing.T) {
		productID := int64(4) // 與之前設定匹配的商品ID

		// 使用商品ID查詢折扣
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 0, []int64{productID})
		assert.NoError(t, err)

		// 找到買一送一折扣
		var bogoDiscount *models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Name == "Buy One Get One Free" {
				bogoDiscount = &availableDiscounts[i]
				break
			}
		}

		assert.NotNil(t, bogoDiscount, "應該找到買一送一折扣")
		assert.Equal(t, models.BOGO, bogoDiscount.Type)

		// 模擬購買計算的邏輯，使用折扣物件中的值
		quantity := 4
		unitPrice := 100.0
		regularPrice := float64(quantity) * unitPrice

		// 實際根據折扣規則計算折扣價格
		var discountedPrice float64
		if bogoDiscount.Type == models.BOGO && bogoDiscount.Value == 1.0 {
			// 買N送N的邏輯 (這裡是買1送1)
			discountedPrice = float64(quantity/2+quantity%2) * unitPrice
		}

		assert.Equal(t, 400.0, regularPrice)
		assert.Equal(t, 200.0, discountedPrice)
	})

	// 5. 測試多件折扣
	t.Run("Multi-Item Discount", func(t *testing.T) {
		productID := int64(5) // 與之前設定匹配的商品ID

		// 使用商品ID查詢折扣
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 0, []int64{productID})
		assert.NoError(t, err)

		// 找到多件折扣
		var multiItemDiscount *models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Name == "Multi-Item Discount" {
				multiItemDiscount = &availableDiscounts[i]
				break
			}
		}

		assert.NotNil(t, multiItemDiscount, "應該找到多件折扣")
		assert.Equal(t, models.MultiItem, multiItemDiscount.Type)

		// 模擬購買計算的邏輯，使用折扣物件中的值
		quantity := 3
		unitPrice := 100.0
		regularPrice := float64(quantity) * unitPrice

		// 實際根據折扣規則計算折扣價格
		var discountedPrice float64
		if multiItemDiscount.Type == models.MultiItem {
			// 第二件特定折扣的邏輯 (這裡是第二件5折)
			discountFactor := multiItemDiscount.Value / 100.0

			// 計算每一件商品應用的價格
			prices := make([]float64, quantity)
			for i := 0; i < quantity; i++ {
				if (i+1)%2 == 0 { // 第二件、第四件...
					prices[i] = unitPrice * discountFactor
				} else {
					prices[i] = unitPrice
				}
			}

			// 累加所有商品價格
			for _, price := range prices {
				discountedPrice += price
			}
		}

		assert.Equal(t, 300.0, regularPrice)
		assert.Equal(t, 250.0, discountedPrice)
	})

	// 測試折扣的使用次數限制
	t.Run("Usage Limits", func(t *testing.T) {
		// 先找到百分比折扣
		var percentageDiscount models.Discount
		db.Where("name = ?", "Percentage Discount").First(&percentageDiscount)

		// 更新使用次數到最大限制
		for i := 0; i < 100; i++ {
			err := service.UpdateDiscountUsage(context.Background(), []int64{percentageDiscount.ID})
			assert.NoError(t, err)
		}

		// 再次獲取折扣，應該不會找到此折扣
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)

		found := false
		for _, d := range availableDiscounts {
			if d.Name == "Percentage Discount" {
				found = true
				break
			}
		}
		assert.False(t, found, "達到使用次數上限後不應該找到該折扣")
	})
}

// 新增一個輔助方法，用於計算應用折扣後的金額
func calculateDiscountedAmount(discount *models.Discount, amount float64, quantity int, unitPrice float64) float64 {
	switch discount.Type {
	case models.Percentage:
		return amount * (1 - discount.Value/100)
	case models.Fixed:
		return amount - discount.Value
	case models.Threshold:
		return amount - discount.Value
	case models.BOGO:
		// 買N送N的邏輯
		return float64(quantity/2+quantity%2) * unitPrice
	case models.MultiItem:
		// 第二件特定折扣
		discountFactor := discount.Value / 100.0
		var total float64
		for i := 0; i < quantity; i++ {
			if (i+1)%2 == 0 { // 第二件
				total += unitPrice * discountFactor
			} else {
				total += unitPrice
			}
		}
		return total
	default:
		return amount
	}
}

// 測試折扣優先權機制
func TestDiscountPriority(t *testing.T) {
	db := setupTestDB(t)
	service := NewDiscountService(db)
	now := time.Now()

	// 創建多個優先級不同的折扣
	discounts := []*models.Discount{
		{
			Name:      "Low Priority Discount",
			Type:      models.Percentage,
			Value:     5.0, // 5% 折扣
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  models.PriorityLow, // 優先級低
			Stackable: false,
			MaxUsage:  100,
		},
		{
			Name:      "Medium Priority Discount",
			Type:      models.Percentage,
			Value:     10.0, // 10% 折扣
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  models.PriorityMedium, // 優先級中
			Stackable: false,
			MaxUsage:  100,
		},
		{
			Name:      "High Priority Discount",
			Type:      models.Percentage,
			Value:     15.0, // 15% 折扣
			StartDate: now.Add(-1 * time.Hour),
			EndDate:   now.Add(24 * time.Hour),
			Priority:  models.PriorityHigh, // 優先級高
			Stackable: false,
			MaxUsage:  100,
		},
	}

	// 創建相同條件的折扣，讓它們都適用於同一購物車
	for _, discount := range discounts {
		err := service.CreateDiscount(context.Background(), discount)
		assert.NoError(t, err)

		// 給所有折扣添加相同的條件
		condition := &models.DiscountCondition{
			DiscountID: discount.ID,
			Type:       models.CartTotal,
			Value:      "100", // 購物車總額超過100就適用
		}
		db.Create(condition)
	}

	// 1. 測試基本優先級排序 (低數字優先級高)
	t.Run("Basic Priority Ordering", func(t *testing.T) {
		// 獲取可用折扣，應該按優先級排序
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)
		assert.Len(t, availableDiscounts, 3, "應該找到所有三個折扣")

		// 驗證優先級排序 (高優先級應該排在前面)
		assert.Equal(t, "High Priority Discount", availableDiscounts[0].Name, "高優先級折扣應該排第一")
		assert.Equal(t, "Medium Priority Discount", availableDiscounts[1].Name, "中優先級折扣應該排第二")
		assert.Equal(t, "Low Priority Discount", availableDiscounts[2].Name, "低優先級折扣應該排第三")
	})

	// 2. 測試相同優先級但可疊加性不同的排序
	t.Run("Same Priority Different Stackability", func(t *testing.T) {
		// 創建兩個相同優先級但可疊加性不同的折扣
		samePriorityDiscounts := []*models.Discount{
			{
				Name:      "Non-Stackable Discount",
				Type:      models.Percentage,
				Value:     8.0,
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				Priority:  models.PriorityMedium,
				Stackable: false, // 不可疊加
				MaxUsage:  100,
			},
			{
				Name:      "Stackable Discount",
				Type:      models.Percentage,
				Value:     8.0,
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				Priority:  models.PriorityMedium,
				Stackable: true, // 可疊加
				MaxUsage:  100,
			},
		}

		for _, discount := range samePriorityDiscounts {
			err := service.CreateDiscount(context.Background(), discount)
			assert.NoError(t, err)

			condition := &models.DiscountCondition{
				DiscountID: discount.ID,
				Type:       models.CartTotal,
				Value:      "100",
			}
			db.Create(condition)
		}

		// 獲取可用折扣，檢查排序
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)

		// 找出兩個相同優先級的折扣的排序
		var nonStackablePos, stackablePos int
		for i, d := range availableDiscounts {
			if d.Name == "Non-Stackable Discount" {
				nonStackablePos = i
			}
			if d.Name == "Stackable Discount" {
				stackablePos = i
			}
		}

		// 驗證不可疊加的折扣優先級較高 (排序較前)
		assert.True(t, nonStackablePos < stackablePos, "不可疊加的折扣應排在可疊加折扣之前")
	})

	// 3. 測試有多個同類型折扣時的應用邏輯
	t.Run("Multiple Applicable Discounts", func(t *testing.T) {
		// 只測試第一個（最高優先級）折扣實際計算的金額
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)
		assert.True(t, len(availableDiscounts) > 0, "應該至少找到一個折扣")

		// 取最高優先級的折扣
		highestPriorityDiscount := availableDiscounts[0]
		assert.Equal(t, "High Priority Discount", highestPriorityDiscount.Name)
		assert.Equal(t, models.Percentage, highestPriorityDiscount.Type)
		assert.Equal(t, 15.0, highestPriorityDiscount.Value)

		// 使用最高優先級折扣計算折扣金額
		totalAmount := 200.0

		// 使用輔助函數計算折扣金額
		discountedAmount := calculateDiscountedAmount(&highestPriorityDiscount, totalAmount, 0, 0)

		// 驗證使用了最高優先級的折扣 (15%)
		expectedAmount := 200.0 * (1 - 15.0/100) // 200 * 0.85 = 170
		assert.Equal(t, expectedAmount, discountedAmount)

		// 如果應用了次高優先級折扣 (10%) 會是什麼結果
		mediumPriorityAmount := 200.0 * (1 - 10.0/100) // 200 * 0.9 = 180
		// 確認實際折扣金額不等於使用次高優先級計算的結果
		assert.NotEqual(t, mediumPriorityAmount, discountedAmount)
	})

	// 4. 測試可疊加折扣
	t.Run("Stackable Discounts", func(t *testing.T) {
		// 清除之前的折扣以便測試
		db.Exec("DELETE FROM discount_conditions")
		db.Exec("DELETE FROM discounts")

		// 創建一些可疊加的折扣
		stackableDiscounts := []*models.Discount{
			{
				Name:      "Stackable Discount 1",
				Type:      models.Percentage,
				Value:     10.0, // 10% 折扣
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				Priority:  models.PriorityHigh,
				Stackable: true, // 可疊加
				MaxUsage:  100,
			},
			{
				Name:      "Stackable Discount 2",
				Type:      models.Fixed,
				Value:     20.0, // 固定減$20
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				Priority:  models.PriorityMedium,
				Stackable: true, // 可疊加
				MaxUsage:  100,
			},
			{
				Name:      "Non-Stackable Discount",
				Type:      models.Percentage,
				Value:     15.0, // 15% 折扣
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(24 * time.Hour),
				Priority:  models.PriorityLow,
				Stackable: false, // 不可疊加
				MaxUsage:  100,
			},
		}

		for _, discount := range stackableDiscounts {
			err := service.CreateDiscount(context.Background(), discount)
			assert.NoError(t, err)

			condition := &models.DiscountCondition{
				DiscountID: discount.ID,
				Type:       models.CartTotal,
				Value:      "100",
			}
			db.Create(condition)
		}

		// 獲取可用折扣
		availableDiscounts, err := service.GetAvailableDiscounts(context.Background(), 0, 200, []int64{})
		assert.NoError(t, err)
		assert.Len(t, availableDiscounts, 3, "應該找到所有三個折扣")

		// 模擬購物車總金額
		totalAmount := 200.0

		// 模擬結帳流程中的折扣計算邏輯
		// 1. 先應用不可疊加的折扣 (找到優先級最高的)
		// 2. 再應用所有可疊加的折扣

		// 找出不可疊加的折扣中優先級最高的
		var highestNonStackable *models.Discount
		for i := range availableDiscounts {
			if !availableDiscounts[i].Stackable {
				if highestNonStackable == nil || availableDiscounts[i].Priority < highestNonStackable.Priority {
					highestNonStackable = &availableDiscounts[i]
				}
			}
		}

		// 應用不可疊加的折扣
		var finalAmount float64
		if highestNonStackable != nil {
			finalAmount = calculateDiscountedAmount(highestNonStackable, totalAmount, 0, 0)
		} else {
			finalAmount = totalAmount
		}

		// 應用可疊加的折扣 (按優先級排序)
		var stackableDiscountsList []*models.Discount
		for i := range availableDiscounts {
			if availableDiscounts[i].Stackable {
				stackableDiscountsList = append(stackableDiscountsList, &availableDiscounts[i])
			}
		}

		// 按優先級排序
		sort.Slice(stackableDiscountsList, func(i, j int) bool {
			return stackableDiscountsList[i].Priority < stackableDiscountsList[j].Priority
		})

		// 應用可疊加折扣
		for _, discount := range stackableDiscountsList {
			finalAmount = calculateDiscountedAmount(discount, finalAmount, 0, 0)
		}

		// 驗證最終金額
		// 預期計算過程：
		// 1. 應用非疊加折扣: 200 * (1 - 15/100) = 170
		// 2. 應用第一個可疊加折扣: 170 * (1 - 10/100) = 153
		// 3. 應用第二個可疊加折扣: 153 - 20 = 133
		expectedFinalAmount := 200.0 * (1 - 15.0/100)              // 應用 15% 折扣 => 170
		expectedFinalAmount = expectedFinalAmount * (1 - 10.0/100) // 應用 10% 折扣 => 153
		expectedFinalAmount = expectedFinalAmount - 20.0           // 應用固定折扣 $20 => 133

		assert.InDelta(t, expectedFinalAmount, finalAmount, 0.01, "折扣計算結果不正確")
	})
}
