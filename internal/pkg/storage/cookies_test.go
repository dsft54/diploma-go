package storage

import (
	"net/http"
	"testing"
)

func TestCookieStorage_CheckIfValid(t *testing.T) {
	case1 := NewCS(6)
	case1.AddCookie(&http.Cookie{
		Name:   "testUser",
		Value:  "12345678",
		MaxAge: 3600,
	})
	case2 := NewCS(6)
	tests := []struct {
		name      string
		cs        *CookieStorage
		coo       *http.Cookie
		wantValid bool
	}{
		{
			name: "Normal",
			cs:   case1,
			coo: &http.Cookie{
				Name:   "testUser",
				Value:  "12345678",
				MaxAge: 3600,
			},
			wantValid: true,
		},
		{
			name: "Empty",
			cs:   case2,
			coo: &http.Cookie{
				Name:   "testUser",
				Value:  "12345678",
				MaxAge: 3600,
			},
			wantValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotValid := tt.cs.CheckIfValid(tt.coo); gotValid != tt.wantValid {
				t.Errorf("CookieStorage.CheckIfValid() = %v, want %v", gotValid, tt.wantValid)
			}
		})
	}
}

func TestCookieStorage_GetUserbyCookie(t *testing.T) {
	testCS := NewCS(6)
	testCS.AddCookie(&http.Cookie{
		Name:   "testUser",
		Value:  "12345678",
		MaxAge: 3600,
	})
	tests := []struct {
		name  string
		cs    *CookieStorage
		value string
		want  string
	}{
		{
			name: "Normal",
			cs: testCS,
			value: "12345678",
			want: "testUser",
		},
		{
			name: "Not exists",
			cs: testCS,
			value: "87654321",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cs.GetUserbyCookie(tt.value); got != tt.want {
				t.Errorf("CookieStorage.GetUserbyCookie() = %v, want %v", got, tt.want)
			}
		})
	}
}
