package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/products"
)

func toProductRecord(r products.Row) products.Row { return r }

func ProductsList(c *gin.Context) {
	slug := strings.TrimSpace(c.Query("slug"))
	ctx := c.Request.Context()

	if slug != "" {
		row, err := products.SelectBySlug(ctx, slug)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to fetch products: "+err.Error())
			return
		}
		if row == nil || !row.Active {
			c.JSON(http.StatusNotFound, gin.H{"product": nil})
			return
		}
		c.JSON(http.StatusOK, gin.H{"product": row})
		return
	}

	rows, err := products.SelectActive(ctx)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch products: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"products": rows})
}

func ProductsAdminList(c *gin.Context) {
	rows, err := products.SelectAll(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch products: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"products": rows})
}

func ProductsAdminCreate(c *gin.Context) {
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid product data. Required: slug, nameUk, nameEn, pricingType ('fixed'|'on_request'); fixed products require priceUahMinor and priceUsdMinor.")
		return
	}
	input := products.ParseCreateInput(raw)
	if input == nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid product data. Required: slug, nameUk, nameEn, pricingType ('fixed'|'on_request'); fixed products require priceUahMinor and priceUsdMinor.")
		return
	}
	row, err := products.Insert(c.Request.Context(), *input)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
			httpx.Error(c, http.StatusConflict, "A product with this slug already exists.")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "Failed to create product: "+msg)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"product": row})
}

func ProductsAdminUpdate(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		httpx.Error(c, http.StatusBadRequest, "Missing product id.")
		return
	}
	var raw map[string]any
	if err := c.ShouldBindJSON(&raw); err != nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid update data.")
		return
	}
	input := products.ParseUpdateInput(raw)
	if input == nil {
		httpx.Error(c, http.StatusBadRequest, "Invalid update data.")
		return
	}
	row, err := products.Update(c.Request.Context(), id, *input)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate") {
			httpx.Error(c, http.StatusConflict, "A product with this slug already exists.")
			return
		}
		if strings.Contains(msg, "products_fixed_prices_required") {
			httpx.Error(c, http.StatusBadRequest, "Fixed-price products require both priceUahMinor and priceUsdMinor.")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "Failed to update product: "+msg)
		return
	}
	if row == nil {
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"product": row})
}

func ProductsAdminDelete(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		httpx.Error(c, http.StatusBadRequest, "Missing product id.")
		return
	}
	ctx := c.Request.Context()
	existing, err := products.SelectByID(ctx, id)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to delete product: "+err.Error())
		return
	}
	if existing == nil {
		httpx.Error(c, http.StatusNotFound, "Product not found.")
		return
	}
	if _, err := products.Delete(ctx, id); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "foreign key") || strings.Contains(msg, "referenced") {
			httpx.Error(c, http.StatusConflict, "Cannot delete product: it has associated payments.")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "Failed to delete product: "+msg)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
