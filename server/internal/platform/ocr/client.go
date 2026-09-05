package ocr

import (
	"context"
	"io"
)

type Client interface {
	ExtractText(ctx context.Context, image io.Reader, contentType string) (TextResult, error)
}

type TextLine struct {
	Text       string
	Confidence float64
}

type TextResult struct {
	Provider   string
	Action     string
	Text       string
	Lines      []TextLine
	Confidence float64
}
