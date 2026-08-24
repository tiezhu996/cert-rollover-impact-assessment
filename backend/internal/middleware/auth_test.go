package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimiterFirstRequestNoPanic(t *testing.T) {
	limiter := NewRateLimiter(5)
	router := gin.New()
	router.Use(limiter.Middleware("login"))
	router.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestLoginRateLimiterConcurrentWritesNoRace(t *testing.T) {
	limiter := NewRateLimiter(100)
	router := gin.New()
	router.Use(limiter.Middleware("login"))
	router.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			req := httptest.NewRequest(http.MethodGet, "/ping", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusOK && w.Code != http.StatusTooManyRequests {
				t.Errorf("unexpected status %d", w.Code)
			}
		}()
	}
	close(start)
	wg.Wait()
}

func TestRateLimiterClientsIndependent(t *testing.T) {
	limiter := NewRateLimiter(3)
	router := gin.New()
	router.Use(limiter.Middleware("login"))
	router.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	clientA := "10.0.0.1:1234"
	clientB := "10.0.0.2:1234"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = clientA
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("client A request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = clientB
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("client B should not share client A's budget, got %d", w.Code)
	}
}

func TestRateLimiterAllowsExactLimit(t *testing.T) {
	limiter := NewRateLimiter(3)
	router := gin.New()
	router.Use(limiter.Middleware("login"))
	router.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d within limit: expected 200, got %d", i+1, w.Code)
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request over limit: expected 429, got %d", w.Code)
	}
}
