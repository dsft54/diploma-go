package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"diploma/cmd/storage"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
)

func TestPingDB(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		method string
		store  *storage.Storage
		want   int
	}{
		{
			name:   "Normal condition",
			uri:    "postgres://postgres:example@localhost:5432",
			method: "GET",
			want:   200,
		},
		{
			name:   "Store connection failed (wrong uri)",
			uri:    "postgres://postgres:example@localhost:543",
			method: "GET",
			want:   500,
		},
	}
	for _, tt := range tests {
		tt.store, _ = storage.NewStorageConnection(context.Background(), tt.uri)
		w := httptest.NewRecorder()
		_, r := gin.CreateTestContext(w)
		r.GET("/ping", PingDB(tt.store))
		req, _ := http.NewRequest(tt.method, "/ping", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, tt.want, w.Code)
	}
}
