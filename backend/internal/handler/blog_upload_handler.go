package handler

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// BlogUploadHandler handles admin blog image uploads (cover & rich-text content images)
// and serves them publicly. Stored under {dataDir}/uploads/blogs/.
type BlogUploadHandler struct {
	uploadsDir string
}

const maxBlogImageSize = 10 << 20 // 10MB

// extByContentType maps sniffed content types to safe extensions.
var extByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// NewBlogUploadHandler creates the handler and ensures the uploads directory exists.
func NewBlogUploadHandler(dataDir string) *BlogUploadHandler {
	dir := filepath.Join(dataDir, "uploads", "blogs")
	_ = os.MkdirAll(dir, 0o755)
	return &BlogUploadHandler{uploadsDir: dir}
}

// RegisterBlogUploadRoutes registers admin upload + public image serving routes.
// POST /api/v1/admin/uploads/image  (admin auth, audit, compliance guard)
// GET  /api/v1/public/uploads/blogs/*filename (no auth: browser img tags cannot carry tokens)
func RegisterBlogUploadRoutes(
	v1 *gin.RouterGroup,
	dataDir string,
	adminAuth gin.HandlerFunc,
	auditLog gin.HandlerFunc,
	settingService *service.SettingService,
) {
	h := NewBlogUploadHandler(dataDir)

	admin := v1.Group("/admin/uploads")
	admin.Use(adminAuth)
	admin.Use(gin.HandlerFunc(auditLog))
	admin.Use(middleware2.AdminComplianceGuard(settingService))
	{
		admin.POST("/image", h.UploadImage)
	}

	pub := v1.Group("/public/uploads")
	{
		pub.GET("/blogs/*filename", h.ServeImage)
	}
}

// UploadImage handles multipart image upload.
// POST /api/v1/admin/uploads/image  (form field: "file")
func (h *BlogUploadHandler) UploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "missing file field")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxBlogImageSize {
		response.Error(c, http.StatusRequestEntityTooLarge, "image too large (max 10MB)")
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		response.InternalError(c, "failed to read upload")
		return
	}
	defer src.Close()

	// Sniff real content type (don't trust client header / extension).
	data, err := io.ReadAll(src)
	if err != nil {
		response.BadRequest(c, "failed to read upload")
		return
	}
	if len(data) == 0 {
		response.BadRequest(c, "empty file")
		return
	}
	sniffed := http.DetectContentType(data)
	ext, ok := extByContentType[sniffed]
	if !ok {
		response.BadRequest(c, "unsupported image type (jpeg/png/gif/webp only)")
		return
	}

	name := randomHex(16) + ext
	dstPath := filepath.Join(h.uploadsDir, name)
	if err := os.WriteFile(dstPath, data, 0o644); err != nil {
		response.InternalError(c, "failed to store upload")
		return
	}

	url := "/api/v1/public/uploads/blogs/" + name
	response.Success(c, gin.H{"url": url})
}

// ServeImage serves uploaded blog images.
// GET /api/v1/public/uploads/blogs/*filename
func (h *BlogUploadHandler) ServeImage(c *gin.Context) {
	filename := strings.TrimPrefix(c.Param("filename"), "/")
	// Uploads are stored flat: reject anything that is not a plain filename.
	if filename == "" ||
		strings.Contains(filename, "\\") ||
		strings.Contains(filename, "/") ||
		strings.Contains(filename, "..") ||
		strings.ContainsRune(filename, 0) {
		response.NotFound(c, "image not found")
		return
	}

	cleaned := filepath.Clean(filepath.Join(h.uploadsDir, filename))
	if !strings.HasPrefix(cleaned, filepath.Clean(h.uploadsDir)+string(filepath.Separator)) {
		response.NotFound(c, "image not found")
		return
	}

	info, err := os.Stat(cleaned)
	if err != nil || info.IsDir() {
		response.NotFound(c, "image not found")
		return
	}

	// Uploaded images are immutable (random names); allow long caching.
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(cleaned)
}

// UploadImageForBlogAPI exposes image upload under /v1/blog using the same
// sk- API Key auth that blog create uses. Only admin-role keys are allowed.
func (h *BlogUploadHandler) UploadImageForBlogAPI(c *gin.Context) {
	role, ok := middleware2.GetUserRoleFromContext(c)
	if !ok || role != service.RoleAdmin {
		response.Error(c, http.StatusForbidden, "admin access required")
		return
	}
	h.UploadImage(c)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand practically never fails; panic to surface the environment issue.
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
