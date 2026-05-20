package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/auth"
	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/lectures"
	"github.com/apexwoot/lms-sls-go/internal/userfeatures"
)

const lecturesFeature = "lectures"

func UserLectures(c *gin.Context) {
	user := auth.UserFrom(c)
	ctx := c.Request.Context()

	hasAccess, err := userfeatures.HasActiveFeature(ctx, user.UserID, lecturesFeature)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lectures: "+err.Error())
		return
	}
	if !hasAccess {
		httpx.Error(c, http.StatusForbidden, "No access to lectures.")
		return
	}

	slug := strings.TrimSpace(c.Query("slug"))
	if slug == "" {
		list, err := lectures.SelectActive(ctx)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lectures: "+err.Error())
			return
		}
		c.JSON(http.StatusOK, gin.H{"lectures": list})
		return
	}

	lecture, err := lectures.SelectBySlug(ctx, slug)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lectures: "+err.Error())
		return
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}

	watermarked, err := lectures.ApplyWatermark(lecture.PDFData)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to watermark lecture: "+err.Error())
		return
	}

	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Disposition", `inline; filename="`+slug+`.pdf"`)
	c.Data(http.StatusOK, "application/pdf", watermarked)
}
