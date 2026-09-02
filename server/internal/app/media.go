package app

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/chenbb0128/tuoguan-system-server/internal/platform/storage"
)

// protectedUploadHandler serves uploaded images only through short-lived signed
// URLs. The mini program cannot reliably attach an Authorization header to an
// image request, so the signature is carried in the query string instead.
func protectedUploadHandler(signer *storage.URLSigner, files storage.FileReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if !strings.HasPrefix(requestPath, "/uploads/") || !signer.Verify(requestPath, c.Query("expires"), c.Query("sig"), time.Now()) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		if files == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		reader, contentType, err := files.OpenURL(c.Request.URL.RequestURI())
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if closer, ok := reader.(io.Closer); ok {
			defer func() { _ = closer.Close() }()
		}
		if contentType != "" {
			c.Header("Content-Type", contentType)
		}
		http.ServeContent(c.Writer, c.Request, path.Base(requestPath), time.Time{}, reader)
	}
}
