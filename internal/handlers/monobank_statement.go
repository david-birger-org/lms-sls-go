package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/monobank"
)

func MonobankStatement(c *gin.Context) {
	r, err := monobank.ParseStatementRange(c.Request.URL.Query())
	if err != nil {
		var rangeErr *monobank.InvalidStatementRangeError
		if errors.As(err, &rangeErr) {
			httpx.Error(c, http.StatusBadRequest, rangeErr.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, "Failed to load statement: "+err.Error())
		return
	}
	client := monobank.NewClient()
	items, err := client.FetchStatement(c.Request.Context(), r)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to load statement: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": items})
}
