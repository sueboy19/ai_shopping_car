package handlers

import (
	"net/http"
	"strconv"

	"shopping_cart/models"
	"shopping_cart/services"

	"github.com/gin-gonic/gin"
)

type DiscountHandler struct {
	discountService *services.DiscountService
}

func NewDiscountHandler(discountService *services.DiscountService) *DiscountHandler {
	return &DiscountHandler{discountService: discountService}
}

func (h *DiscountHandler) CreateDiscount(c *gin.Context) {
	var discount models.Discount
	if err := c.ShouldBindJSON(&discount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.discountService.CreateDiscount(c.Request.Context(), &discount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, discount)
}

func (h *DiscountHandler) UpdateDiscount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var discount models.Discount
	if err := c.ShouldBindJSON(&discount); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.discountService.UpdateDiscount(c.Request.Context(), id, &discount); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, discount)
}

func (h *DiscountHandler) DeleteDiscount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.discountService.DeleteDiscount(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *DiscountHandler) GetAvailableDiscounts(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	cartTotal, _ := strconv.ParseFloat(c.Query("cart_total"), 64)
	productIDsStr := c.QueryArray("product_ids")
	productIDs := make([]int64, len(productIDsStr))
	for i, idStr := range productIDsStr {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product id"})
			return
		}
		productIDs[i] = id
	}

	discounts, err := h.discountService.GetAvailableDiscounts(c.Request.Context(), userID, cartTotal, productIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, discounts)
}
