package main

import (
	"log"
	"shopping_cart/handlers"
	"shopping_cart/models"
	"shopping_cart/services"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// 初始化數據庫連接
	db, err := gorm.Open(sqlite.Open("file:shopping_cart.db?cache=shared&mode=memory"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// 自動遷移數據庫表
	if err := db.AutoMigrate(
		&models.Discount{},
		&models.DiscountCondition{},
		&models.DiscountProduct{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	// 初始化服務層
	discountService := services.NewDiscountService(db)

	// 初始化路由
	r := gin.Default()
	discountHandler := handlers.NewDiscountHandler(discountService)

	// 設置折扣相關路由
	discountRoutes := r.Group("/discounts")
	{
		discountRoutes.POST("", discountHandler.CreateDiscount)
		discountRoutes.PUT("/:id", discountHandler.UpdateDiscount)
		discountRoutes.DELETE("/:id", discountHandler.DeleteDiscount)
		discountRoutes.GET("", discountHandler.GetAvailableDiscounts)
	}

	// 啟動服務器
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
