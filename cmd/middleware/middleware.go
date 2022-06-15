package middleware

import (
	"bytes"
	"compress/gzip"
	"diploma/cmd/storage"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type gzipBodyWriter struct {
	gin.ResponseWriter
	writer io.Writer
}

func (gz gzipBodyWriter) Write(b []byte) (int, error) {
	return gz.writer.Write(b)
}

func Compression() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		gz, err := gzip.NewWriterLevel(c.Writer, gzip.BestSpeed)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer gz.Close()

		c.Writer = gzipBodyWriter{c.Writer, gz}
		c.Writer.Header().Set("Content-Encoding", "gzip")
		c.Next()
	}
}

func Decompression() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.Request.Header.Get("Content-Encoding"), "gzip") ||
			!strings.Contains(c.Request.Header.Get("Content-Encoding"), "deflate") ||
			!strings.Contains(c.Request.Header.Get("Content-Encoding"), "br") {
			c.Next()
			return
		}
		gz, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		defer gz.Close()

		body, err := io.ReadAll(gz)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Request.ContentLength = int64(len(body))
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
		c.Next()
	}
}

func Authentication(cs *storage.CookieStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.RequestURI == "/api/user/register" ||
			c.Request.RequestURI == "/api/user/login" {
			c.Next()
			return
		}
		if len(c.Request.Cookies()) == 0 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		var valid bool
		for _, requestCookie := range c.Request.Cookies() {
			valid = cs.CheckIfValid(requestCookie)
		}
		if !valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
