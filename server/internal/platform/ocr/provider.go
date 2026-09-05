package ocr

import (
	"fmt"
	"strings"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

func NewClient(cfg config.OCRConfig) (Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "rapidocr"
	}
	switch provider {
	case "rapidocr":
		return NewRapidClient(cfg)
	case "tencent":
		return NewTencentClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported OCR provider %q", provider)
	}
}
