package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// countryBlocklistHandlerRepo 内存版 SettingRepository（bare 层自包含）。
type countryBlocklistHandlerRepo struct {
	mu     sync.Mutex
	values map[string]string
}

func (r *countryBlocklistHandlerRepo) Get(_ context.Context, key string) (*service.Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return nil, service.ErrSettingNotFound
	}
	return &service.Setting{Key: key, Value: value}, nil
}

func (r *countryBlocklistHandlerRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *countryBlocklistHandlerRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func (r *countryBlocklistHandlerRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *countryBlocklistHandlerRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.values == nil {
		r.values = make(map[string]string)
	}
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *countryBlocklistHandlerRepo) GetAll(_ context.Context) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *countryBlocklistHandlerRepo) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.values, key)
	return nil
}

type stubGeoIPStatus struct{ ready bool }

func (s stubGeoIPStatus) Available() bool { return s.ready }

func newCountryBlocklistHandler(ready bool) (*SettingHandler, *countryBlocklistHandlerRepo) {
	repo := &countryBlocklistHandlerRepo{}
	svc := service.NewSettingService(repo, &config.Config{})
	h := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)
	h.SetGeoIPStatus(stubGeoIPStatus{ready: ready})
	return h, repo
}

type countryBlocklistResponse struct {
	Data struct {
		Enabled    bool     `json:"enabled"`
		Countries  []string `json:"countries"`
		GeoIPReady bool     `json:"geoip_ready"`
	} `json:"data"`
}

func TestGetSecurityCountryBlocklistReportsGeoIPReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, ready := range []bool{false, true} {
		h, _ := newCountryBlocklistHandler(ready)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

		h.GetSecurityCountryBlocklist(c)
		require.Equal(t, http.StatusOK, w.Code)

		var body countryBlocklistResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, ready, body.Data.GeoIPReady, "geoip_ready 必须反映真实 GeoIP 就绪状态")
		require.False(t, body.Data.Enabled, "键缺失时默认关闭")
		require.Empty(t, body.Data.Countries, "键缺失时默认空名单")
	}
}

func TestUpdateSecurityCountryBlocklistNormalizes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, repo := newCountryBlocklistHandler(true)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/",
		strings.NewReader(`{"enabled":true,"countries":[" cn ","US","cn","de"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateSecurityCountryBlocklist(c)
	require.Equal(t, http.StatusOK, w.Code)

	var body countryBlocklistResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.True(t, body.Data.Enabled)
	require.Equal(t, []string{"CN", "US", "DE"}, body.Data.Countries)
	require.True(t, body.Data.GeoIPReady)

	stored, err := repo.GetValue(context.Background(), service.SettingKeySecurityCountryBlocklist)
	require.NoError(t, err)
	require.Contains(t, stored, `"CN"`)
}

func TestUpdateSecurityCountryBlocklistRejectsInvalidCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, repo := newCountryBlocklistHandler(false)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/",
		strings.NewReader(`{"enabled":true,"countries":["CHN"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.UpdateSecurityCountryBlocklist(c)
	require.Equal(t, http.StatusBadRequest, w.Code)

	_, err := repo.GetValue(context.Background(), service.SettingKeySecurityCountryBlocklist)
	require.ErrorIs(t, err, service.ErrSettingNotFound, "校验失败绝不能写设置表")
}
