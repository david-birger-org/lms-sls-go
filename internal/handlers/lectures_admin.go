package handlers

import (
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/apexwoot/lms-sls-go/internal/httpx"
	"github.com/apexwoot/lms-sls-go/internal/lectures"
)

// maxNotesBytes stays under Vercel's 4.5 MB request body limit.
const maxNotesBytes = 4_500_000

const invalidVideoMessage = "Invalid video data. Required: videoId (non-empty, at most 1024 characters)."

const invalidLectureMessage = "Invalid lecture data. Required: slug (lowercase letters, digits and single hyphens), title."

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// lectureID reads ?id= and writes a 400 when it is missing or not a uuid.
func lectureID(c *gin.Context) (string, bool) {
	id := strings.TrimSpace(c.Query("id"))
	if !uuidPattern.MatchString(id) {
		httpx.Error(c, http.StatusBadRequest, "Missing or invalid lecture id.")
		return "", false
	}
	return id, true
}

// loadLecture reads ?id= and the lecture row, writing the error response itself.
func loadLecture(c *gin.Context) (*lectures.Admin, bool) {
	id, ok := lectureID(c)
	if !ok {
		return nil, false
	}
	lecture, err := lectures.SelectAdminByID(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lecture: "+err.Error())
		return nil, false
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return nil, false
	}
	return lecture, true
}

func bindLectureInput(c *gin.Context) (*lectures.Input, bool) {
	var in lectures.Input
	if err := c.ShouldBindJSON(&in); err != nil || !in.Normalize() {
		httpx.Error(c, http.StatusBadRequest, invalidLectureMessage)
		return nil, false
	}
	return &in, true
}

func LecturesAdminList(c *gin.Context) {
	list, err := lectures.SelectAllAdmin(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to fetch lectures: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"lectures": list})
}

func LecturesAdminCreate(c *gin.Context) {
	in, ok := bindLectureInput(c)
	if !ok {
		return
	}
	lecture, err := lectures.Insert(c.Request.Context(), *in)
	if lectures.IsUniqueViolation(err) {
		httpx.Error(c, http.StatusConflict, "Slug already exists.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to create lecture: "+err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"lecture": lecture})
}

func LecturesAdminUpdate(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	in, ok := bindLectureInput(c)
	if !ok {
		return
	}
	lecture, err := lectures.Update(c.Request.Context(), id, *in)
	if lectures.IsUniqueViolation(err) {
		httpx.Error(c, http.StatusConflict, "Slug already exists.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to update lecture: "+err.Error())
		return
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"lecture": lecture})
}

func LecturesAdminDelete(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	deleted, videoID, err := lectures.Delete(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to delete lecture: "+err.Error())
		return
	}
	if !deleted {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "videoId": videoID})
}

func LecturesAdminNotesUpload(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxNotesBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		httpx.Error(c, http.StatusRequestEntityTooLarge, "Notes PDF is too large (max 4.5 MB).")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "Failed to read notes upload.")
		return
	}
	if !lectures.IsPDF(data) {
		httpx.Error(c, http.StatusBadRequest, "Notes must be a non-empty PDF file.")
		return
	}
	lecture, err := lectures.SetNotes(c.Request.Context(), id, data)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to save notes: "+err.Error())
		return
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"lecture": lecture})
}

func LecturesAdminNotesDelete(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	lecture, err := lectures.SetNotes(c.Request.Context(), id, nil)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to delete notes: "+err.Error())
		return
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"lecture": lecture})
}

type videoInput struct {
	VideoID string `json:"videoId"`
}

func LecturesAdminVideoSet(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	var in videoInput
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.Error(c, http.StatusBadRequest, invalidVideoMessage)
		return
	}
	videoID, valid := lectures.NormalizeVideoID(in.VideoID)
	if !valid {
		httpx.Error(c, http.StatusBadRequest, invalidVideoMessage)
		return
	}
	setLectureVideo(c, id, &videoID)
}

func LecturesAdminVideoDelete(c *gin.Context) {
	id, ok := lectureID(c)
	if !ok {
		return
	}
	setLectureVideo(c, id, nil)
}

func setLectureVideo(c *gin.Context, id string, videoID *string) {
	lecture, previous, err := lectures.SetVideoID(c.Request.Context(), id, videoID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "Failed to save video: "+err.Error())
		return
	}
	if lecture == nil {
		httpx.Error(c, http.StatusNotFound, "Lecture not found.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"lecture": lecture, "previousVideoId": previous})
}
