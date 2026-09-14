package repository

import (
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// TestEntToMediaTaskRecordMapsMediaUrls 多张产物 URL 必须从 ent 实体映射进
// service 记录，否则轮询接口拿不到 media_urls 列、退化为只返回首张。
func TestEntToMediaTaskRecordMapsMediaUrls(t *testing.T) {
	mt := &dbent.MediaTask{
		LocalID:   "img_multi",
		PublicModel: "minimax-image-01",
		MediaURL:  "https://cdn.example.com/a.png",
		MediaUrls: []string{"https://cdn.example.com/a.png", "https://cdn.example.com/b.png"},
	}
	rec := entToMediaTaskRecord(mt)
	if rec.MediaURL != mt.MediaURL {
		t.Errorf("MediaURL 映射错误: got %q want %q", rec.MediaURL, mt.MediaURL)
	}
	if len(rec.MediaURLs) != 2 {
		t.Fatalf("MediaURLs 应有 2 张，实际 %d 张", len(rec.MediaURLs))
	}
	if rec.MediaURLs[1] != "https://cdn.example.com/b.png" {
		t.Errorf("MediaURLs[1] 映射错误: %q", rec.MediaURLs[1])
	}
}
