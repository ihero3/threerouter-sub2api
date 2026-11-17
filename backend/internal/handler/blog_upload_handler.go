package handler

import (
	"crypto/rand"
	"encoding/hex"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/draw"

	// 注册解码器，供 image.Decode 使用
	_ "golang.org/x/image/webp"
	_ "image/gif"
)

// BlogUploadHandler handles admin image uploads (blog covers, rich-text content,
// custom home page content) and serves them publicly.
// Originals live under {dataDir}/uploads/blogs/, thumbnails under {dataDir}/uploads/blog-thumbs/.
type BlogUploadHandler struct {
	uploadsDir string
	thumbsDir  string
}

const (
	maxBlogImageSize = 10 << 20 // 10MB
	maxThumbSize     = 320      // 缩略图最长边（px），列表页展示用
)

// extByContentType maps sniffed content types to safe extensions.
var extByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// NewBlogUploadHandler creates the handler and ensures the upload directories exist.
func NewBlogUploadHandler(dataDir string) *BlogUploadHandler {
	dir := filepath.Join(dataDir, "uploads", "blogs")
	_ = os.MkdirAll(dir, 0o755)
	thumbsDir := filepath.Join(dataDir, "uploads", "blog-thumbs")
	_ = os.MkdirAll(thumbsDir, 0o755)
	return &BlogUploadHandler{uploadsDir: dir, thumbsDir: thumbsDir}
}

// RegisterBlogUploadRoutes registers admin upload/list + public image serving routes.
// POST /api/v1/admin/uploads/image          (admin auth, audit, compliance guard)
// GET  /api/v1/admin/uploads/images         (admin auth)
// GET  /api/v1/public/uploads/blogs/*filename (no auth: browser img tags cannot carry tokens)
// GET  /api/v1/public/uploads/thumbs/*filename (no auth, 缩略图，首次访问时生成)
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
		admin.GET("/images", h.ListImages)
	}

	pub := v1.Group("/public/uploads")
	{
		pub.GET("/blogs/*filename", h.ServeImage)
		pub.GET("/thumbs/*filename", h.ServeThumbnail)
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
	// 缩略图生成失败不影响上传结果，首次访问缩略图时会重试。
	_ = h.ensureThumbnail(name)

	url := "/api/v1/public/uploads/blogs/" + name
	response.Success(c, gin.H{"url": url})
}

// ListImages lists uploaded images (newest first) for the admin image library.
// GET /api/v1/admin/uploads/images?limit=100
func (h *BlogUploadHandler) ListImages(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	entries, err := os.ReadDir(h.uploadsDir)
	if err != nil {
		response.InternalError(c, "failed to list images")
		return
	}

	type imageItem struct {
		Name      string `json:"name"`
		URL       string `json:"url"`
		ThumbURL  string `json:"thumb_url"`
		Size      int64  `json:"size"`
		CreatedAt string `json:"created_at"`
	}

	items := make([]imageItem, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !isAllowedImageName(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, imageItem{
			Name:      entry.Name(),
			URL:       publicImageURL(entry.Name()),
			ThumbURL:  publicThumbURL(entry.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}

	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	total := len(items)
	if total > limit {
		items = items[:limit]
	}

	response.Success(c, gin.H{"items": items, "total": total})
}

// ServeImage serves uploaded blog images.
// GET /api/v1/public/uploads/blogs/*filename
func (h *BlogUploadHandler) ServeImage(c *gin.Context) {
	filename := strings.TrimPrefix(c.Param("filename"), "/")
	// Uploads are stored flat: reject anything that is not a plain filename.
	if !isPlainFilename(filename) {
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

// ServeThumbnail serves a downscaled thumbnail, generating it on first access.
// GET /api/v1/public/uploads/thumbs/*filename （filename 为原图文件名）
func (h *BlogUploadHandler) ServeThumbnail(c *gin.Context) {
	filename := strings.TrimPrefix(c.Param("filename"), "/")
	if !isPlainFilename(filename) || !isAllowedImageName(filename) {
		response.NotFound(c, "image not found")
		return
	}
	if _, err := os.Stat(filepath.Join(h.uploadsDir, filename)); err != nil {
		response.NotFound(c, "image not found")
		return
	}

	thumbPath := h.thumbnailPath(filename)
	if _, err := os.Stat(thumbPath); err != nil {
		if err := h.ensureThumbnail(filename); err != nil {
			// 无法生成缩略图时回退到原图，避免列表出现破图。
			c.Redirect(http.StatusTemporaryRedirect, publicImageURL(filename))
			return
		}
	}

	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.File(thumbPath)
}

// ensureThumbnail decodes the original image and writes a downscaled thumbnail.
// Existing thumbnails are reused; failures are returned to the caller.
func (h *BlogUploadHandler) ensureThumbnail(name string) error {
	dstPath := h.thumbnailPath(name)
	if _, err := os.Stat(dstPath); err == nil {
		return nil
	}

	src, err := os.Open(filepath.Join(h.uploadsDir, name))
	if err != nil {
		return err
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return err
	}
	thumb := downscale(img, maxThumbSize)

	tmp, err := os.CreateTemp(h.thumbsDir, "tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if thumbnailExt(name) == ".jpg" {
		err = jpeg.Encode(tmp, thumb, &jpeg.Options{Quality: 82})
	} else {
		err = png.Encode(tmp, thumb)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// 先写临时文件再 rename，避免并发请求读到半张图。
	return os.Rename(tmpPath, dstPath)
}

// thumbnailPath returns the thumbnail file path for an uploaded image name.
func (h *BlogUploadHandler) thumbnailPath(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return filepath.Join(h.thumbsDir, base+".thumb"+thumbnailExt(name))
}

// thumbnailExt picks the thumbnail format: JPEG for JPEG originals (keeps files small),
// PNG otherwise (preserves transparency for logos/screenshots).
func thumbnailExt(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return ".jpg"
	default:
		return ".png"
	}
}

// downscale shrinks src so that its longest side is at most maxSize pixels.
func downscale(src image.Image, maxSize int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return src
	}
	if w <= maxSize && h <= maxSize {
		return src
	}
	scale := float64(maxSize) / float64(max(w, h))
	nw := max(int(float64(w)*scale), 1)
	nh := max(int(float64(h)*scale), 1)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

// isAllowedImageName reports whether the filename has a supported image extension.
func isAllowedImageName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

// isPlainFilename rejects empty names and any path-ish input.
func isPlainFilename(name string) bool {
	return name != "" &&
		!strings.Contains(name, "\\") &&
		!strings.Contains(name, "/") &&
		!strings.Contains(name, "..") &&
		!strings.ContainsRune(name, 0)
}

func publicImageURL(name string) string {
	return "/api/v1/public/uploads/blogs/" + name
}

func publicThumbURL(name string) string {
	return "/api/v1/public/uploads/thumbs/" + name
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
