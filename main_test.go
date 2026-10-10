package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestPresetsCoverSupportedLanguages(t *testing.T) {
	recorder := httptest.NewRecorder()
	handlePresets(recorder, httptest.NewRequest("GET", "/api/presets", nil))

	if recorder.Code != 200 {
		t.Fatalf("Expected HTTP 200, got %d", recorder.Code)
	}

	var presets map[string]struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &presets); err != nil {
		t.Fatalf("Unable to decode preset response: %v", err)
	}

	expected := map[string]string{
		"preset_python":  "python",
		"preset_c":       "c",
		"preset_cpp":     "cpp",
		"preset_java":    "java",
		"preset_js":      "javascript",
		"preset_ts":      "typescript",
		"preset_csharp":  "csharp",
		"preset_go":      "go",
	}
	for presetID, language := range expected {
		preset, ok := presets[presetID]
		if !ok {
			t.Errorf("Missing preset %q", presetID)
			continue
		}
		if preset.Language != language {
			t.Errorf("Preset %q uses language %q, want %q", presetID, preset.Language, language)
		}
		if preset.Code == "" {
			t.Errorf("Preset %q has no source code", presetID)
		}
	}
}
