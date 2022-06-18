package storage

import (
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type CookieStorage struct {
	Stock []*http.Cookie
	RSeed string
	m     sync.RWMutex
}

func NewCS(seedComplexity int) *CookieStorage {
	cs := new(CookieStorage)
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, seedComplexity)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	cs.RSeed = string(b)
	return cs
}

func (cs *CookieStorage) AddCookie(coo *http.Cookie) {
	var index int
	exists := false
	coo.Expires = time.Now().Add(time.Second * time.Duration(coo.MaxAge))
	cs.m.Lock()
	defer cs.m.Unlock()
	for i, v := range cs.Stock {
		if v.Value == coo.Value {
			exists = true
			index = i
		}
	}
	if exists {
		cs.Stock[index].Expires = coo.Expires
	} else {
		cs.Stock = append(cs.Stock, coo)
	}
}

func (cs *CookieStorage) CheckIfValid(coo *http.Cookie) (valid bool) {
	cs.m.RLock()
	defer cs.m.RUnlock()
	for _, v := range cs.Stock {
		if v.Value == coo.Value {
			valid = true
			coo = v
		}
	}
	if !valid {
		return valid
	}
	if coo.Expires.Before(time.Now()) {
		valid = false
		return valid
	}
	return valid
}

func (cs *CookieStorage) GetUserbyCookie(value string) string {
	cs.m.RLock()
	defer cs.m.RUnlock()
	for _, v := range cs.Stock {
		if v.Value == value {
			return v.Name
		}
	}
	return ""
}
