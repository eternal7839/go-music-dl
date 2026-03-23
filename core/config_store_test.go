package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func resetConfigStateForTest() {
	if configDB != nil {
		if sqlDB, err := configDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
	configDB = nil
	configInitErr = nil
	configInit = sync.Once{}

	CM.mu.Lock()
	CM.cookies = make(map[string]string)
	CM.mu.Unlock()
}

func TestCookieManagerMigratesLegacyJSONAndPersistsToSQLite(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	if err := os.MkdirAll(filepath.Join(baseDir, "data"), 0755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	legacy := map[string]string{
		"netease": "foo=bar",
		"qq":      "uin=123",
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy cookies: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "data", "cookies.json"), raw, 0644); err != nil {
		t.Fatalf("write legacy cookies: %v", err)
	}

	CM.Load()
	if got := CM.GetAll(); !reflect.DeepEqual(got, legacy) {
		t.Fatalf("loaded cookies mismatch\ngot:  %#v\nwant: %#v", got, legacy)
	}

	CM.SetAll(map[string]string{
		"netease": "foo=updated",
		"qq":      "",
		"kugou":   "token=456",
	})
	CM.Save()

	resetConfigStateForTest()
	CM.Load()

	want := map[string]string{
		"netease": "foo=updated",
		"kugou":   "token=456",
	}
	if got := CM.GetAll(); !reflect.DeepEqual(got, want) {
		t.Fatalf("reloaded cookies mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "data", "settings.db")); err != nil {
		t.Fatalf("expected sqlite db to exist: %v", err)
	}
}

func TestWebSettingsDefaultAndPersist(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv("MUSIC_DL_CONFIG_DB", filepath.Join(baseDir, "data", "settings.db"))
	t.Setenv("MUSIC_DL_COOKIE_FILE", filepath.Join(baseDir, "data", "cookies.json"))
	resetConfigStateForTest()
	t.Cleanup(resetConfigStateForTest)

	defaults := GetWebSettings()
	if defaults.EmbedDownload {
		t.Fatalf("default EmbedDownload should be false")
	}
	if defaults.DownloadToLocal {
		t.Fatalf("default DownloadToLocal should be false")
	}
	if defaults.DownloadDir != filepath.Clean(DefaultWebDownloadDir) {
		t.Fatalf("default DownloadDir mismatch: got %q want %q", defaults.DownloadDir, filepath.Clean(DefaultWebDownloadDir))
	}

	if err := SaveWebSettings(WebSettings{
		EmbedDownload:   true,
		DownloadToLocal: true,
		DownloadDir:     "",
	}); err != nil {
		t.Fatalf("save web settings: %v", err)
	}

	got := GetWebSettings()
	want := WebSettings{
		EmbedDownload:   true,
		DownloadToLocal: true,
		DownloadDir:     filepath.Clean(DefaultWebDownloadDir),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("saved settings mismatch\ngot:  %#v\nwant: %#v", got, want)
	}

	customDir := filepath.Join("downloads", "custom")
	if err := SaveWebSettings(WebSettings{
		DownloadDir: customDir,
	}); err != nil {
		t.Fatalf("save custom download dir: %v", err)
	}

	got = GetWebSettings()
	if got.DownloadDir != filepath.Clean(customDir) {
		t.Fatalf("custom download dir mismatch: got %q want %q", got.DownloadDir, filepath.Clean(customDir))
	}
}
