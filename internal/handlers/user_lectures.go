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

// requireLecturesFeature writes a 403 or 500 unless the user has the lectures feature.
func requireLecturesFeature(c *gin.Context) bool {
	user := auth.UserFrom(c)
	hasAccess, err := userfeatures.HasActiveFeature(c.Request.Context(), user.UserID, lecturesFeature)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lectures: "+err.Error())
		return false
	}
	if !hasAccess {
		httpx.Error(c, http.StatusForbidden, "No access to lectures.")
		return false
	}
	return true
}

func UserLectures(c *gin.Context) {
	if !requireLecturesFeature(c) {
		return
	}
	ctx := c.Request.Context()

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
	if len(lecture.PDFData) == 0 {
		httpx.Error(c, http.StatusNotFound, "Lecture notes not found.")
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

func UserLectureVideo(c *gin.Context) {
	if !requireLecturesFeature(c) {
		return
	}
	slug := strings.TrimSpace(c.Query("slug"))
	if slug == "" {
		httpx.Error(c, http.StatusBadRequest, "Missing lecture slug.")
		return
	}
	videoID, err := lectures.SelectActiveVideoID(c.Request.Context(), slug)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lecture video: "+err.Error())
		return
	}
	if videoID == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture video not found.")
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.JSON(http.StatusOK, gin.H{"videoId": *videoID})
}
