package handler

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 1x1 red PNG
var testPNG = []byte{
	0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xDE, 0x00, 0x00, 0x00,
	0x0C, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0xF8, 0xCF, 0xC0, 0xF0,
	0x9F, 0x00, 0x05, 0xFE, 0x02, 0xFE, 0xA7, 0x35, 0x81, 0x69, 0x8C, 0x21,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
}

func newUploadContext(t *testing.T, filename string, content []byte) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads/image", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	return c, w
}

func TestBlogUploadHandler_UploadAndServe(t *testing.T) {
	dir := t.TempDir()
	h := NewBlogUploadHandler(dir)

	c, w := newUploadContext(t, "cover.png", testPNG)
	h.UploadImage(c)

	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	idx := strings.Index(body, "/api/v1/public/uploads/blogs/")
	if idx < 0 {
		t.Fatalf("response missing image url: %s", body)
	}
	rest := body[idx+len("/api/v1/public/uploads/blogs/"):]
	name := rest[:strings.IndexAny(rest, `"}`)]
	if !strings.HasSuffix(name, ".png") {
		t.Fatalf("unexpected stored filename: %q", name)
	}
	if _, err := os.Stat(filepath.Join(dir, "uploads", "blogs", name)); err != nil {
		t.Fatalf("uploaded file not stored: %v", err)
	}

	// Serve it back
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/uploads/blogs/"+name, nil)
	c2.Params = gin.Params{{Key: "filename", Value: "/" + name}}
	h.ServeImage(c2)
	if w2.Code != http.StatusOK {
		t.Fatalf("serve status = %d, want 200", w2.Code)
	}
	if !bytes.Equal(w2.Body.Bytes(), testPNG) {
		t.Fatalf("served bytes mismatch (got %d bytes)", w2.Body.Len())
	}
}

func TestBlogUploadHandler_RejectsNonImage(t *testing.T) {
	h := NewBlogUploadHandler(t.TempDir())

	c, w := newUploadContext(t, "evil.txt", []byte("not an image at all"))
	h.UploadImage(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestBlogUploadHandler_RejectsOversize(t *testing.T) {
	h := NewBlogUploadHandler(t.TempDir())

	big := append([]byte{}, testPNG...)
	big = append(big, bytes.Repeat([]byte{0x00}, int(maxBlogImageSize))...)

	c, w := newUploadContext(t, "big.png", big)
	h.UploadImage(c)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413; body=%s", w.Code, w.Body.String())
	}
}

func TestBlogUploadHandler_ServeBlocksTraversal(t *testing.T) {
	dir := t.TempDir()
	h := NewBlogUploadHandler(dir)

	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}

	for _, target := range []string{
		"/../secret.txt",
		"/..%2fsecret.txt",
		"/....//secret.txt",
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/uploads/blogs"+target, nil)
		c.Params = gin.Params{{Key: "filename", Value: target}}
		h.ServeImage(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("traversal %q: status = %d, want 404", target, w.Code)
		}
		if strings.Contains(w.Body.String(), "top secret") {
			t.Fatalf("traversal %q leaked file content", target)
		}
	}
}

func TestBlogUploadHandler_MissingFile(t *testing.T) {
	h := NewBlogUploadHandler(t.TempDir())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/uploads/image", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.UploadImage(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// uploadTestImage uploads content via the handler and returns the stored filename.
func uploadTestImage(t *testing.T, h *BlogUploadHandler, filename string, content []byte) string {
	t.Helper()

	c, w := newUploadContext(t, filename, content)
	h.UploadImage(c)
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	name := strings.TrimPrefix(resp.Data.URL, "/api/v1/public/uploads/blogs/")
	if name == "" || name == resp.Data.URL {
		t.Fatalf("unexpected upload url: %q", resp.Data.URL)
	}
	return name
}

// testGradientPNG builds a w×h PNG so downscaling is actually exercised.
func testGradientPNG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

type listedImage struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	ThumbURL  string `json:"thumb_url"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

func TestBlogUploadHandler_ListImages(t *testing.T) {
	dir := t.TempDir()
	h := NewBlogUploadHandler(dir)

	name := uploadTestImage(t, h, "product.png", testPNG)
	// 非图片文件不应出现在列表中
	if err := os.WriteFile(filepath.Join(dir, "uploads", "blogs", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write non-image: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/uploads/images", nil)
	h.ListImages(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Items []listedImage `json:"items"`
			Total int           `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if resp.Data.Total != 1 || len(resp.Data.Items) != 1 {
		t.Fatalf("items = %d (total %d), want 1", len(resp.Data.Items), resp.Data.Total)
	}
	item := resp.Data.Items[0]
	if item.Name != name {
		t.Fatalf("name = %q, want %q", item.Name, name)
	}
	if item.URL != "/api/v1/public/uploads/blogs/"+name {
		t.Fatalf("url = %q", item.URL)
	}
	if item.ThumbURL != "/api/v1/public/uploads/thumbs/"+name {
		t.Fatalf("thumb_url = %q", item.ThumbURL)
	}
	if item.Size <= 0 || item.CreatedAt == "" {
		t.Fatalf("unexpected item: %+v", item)
	}
}

func TestBlogUploadHandler_ServeThumbnailLazily(t *testing.T) {
	dir := t.TempDir()
	h := NewBlogUploadHandler(dir)

	// 直接写入原图（模拟历史已上传、尚无缩略图的文件），验证访问时懒生成
	const name = "legacy.png"
	if err := os.WriteFile(filepath.Join(dir, "uploads", "blogs", name), testGradientPNG(t, 640, 480), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/uploads/thumbs/"+name, nil)
	c.Params = gin.Params{{Key: "filename", Value: "/" + name}}
	h.ServeThumbnail(c)

	if w.Code != http.StatusOK {
		t.Fatalf("thumb status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content-type = %q, want image/png", ct)
	}
	thumb, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode thumb: %v", err)
	}
	if got := thumb.Bounds().Dx(); got != maxThumbSize {
		t.Fatalf("thumb width = %d, want %d", got, maxThumbSize)
	}
	if _, err := os.Stat(filepath.Join(dir, "uploads", "blog-thumbs", "legacy.thumb.png")); err != nil {
		t.Fatalf("thumbnail not persisted: %v", err)
	}
}

func TestBlogUploadHandler_ServeThumbnailBlocksTraversal(t *testing.T) {
	h := NewBlogUploadHandler(t.TempDir())

	for _, target := range []string{"/../secret.png", "/..%2fsecret.png", "/sub/dir.png"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/public/uploads/thumbs"+target, nil)
		c.Params = gin.Params{{Key: "filename", Value: target}}
		h.ServeThumbnail(c)
		if w.Code != http.StatusNotFound {
			t.Fatalf("traversal %q: status = %d, want 404", target, w.Code)
		}
	}
}
