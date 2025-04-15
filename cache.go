// Package plugin_simplecache is a plugin to cache responses to disk.
package plugin_simplecache

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pquerna/cachecontrol"
)

// Config configures the middleware.
type Config struct {
	Path            string `json:"path" yaml:"path" toml:"path"`
	MaxExpiry       int    `json:"maxExpiry" yaml:"maxExpiry" toml:"maxExpiry"`
	Cleanup         int    `json:"cleanup" yaml:"cleanup" toml:"cleanup"`
	AddStatusHeader bool   `json:"addStatusHeader" yaml:"addStatusHeader" toml:"addStatusHeader"`
}

// CreateConfig returns a config instance.
func CreateConfig() *Config {
	return &Config{
		MaxExpiry:       int((5 * time.Minute).Seconds()),
		Cleanup:         int((5 * time.Minute).Seconds()),
		AddStatusHeader: true,
	}
}

const (
	cacheHeader      = "Cache-Status"
	cacheHitStatus   = "hit"
	cacheMissStatus  = "miss"
	cacheErrorStatus = "error"
)

type cache struct {
	name  string
	cache *fileCache
	cfg   *Config
	next  http.Handler
}

// New returns a plugin instance.
func New(_ context.Context, next http.Handler, cfg *Config, name string) (http.Handler, error) {
	if cfg.MaxExpiry <= 1 {
		return nil, errors.New("maxExpiry must be greater or equal to 1")
	}

	if cfg.Cleanup <= 1 {
		return nil, errors.New("cleanup must be greater or equal to 1")
	}

	fc, err := newFileCache(cfg.Path, time.Duration(cfg.Cleanup)*time.Second)
	if err != nil {
		return nil, err
	}

	m := &cache{
		name:  name,
		cache: fc,
		cfg:   cfg,
		next:  next,
	}

	return m, nil
}

type cacheData struct {
	Status  int
	Headers map[string][]string
	Body    []byte
}

// ServeHTTP serves an HTTP request.
func (m *cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cs := cacheMissStatus

	key := cacheKey(r)

	b, err := m.cache.Get(key)
	if err == nil {
		var data cacheData

		if err := json.Unmarshal(b, &data); err != nil {
			log.Printf("Error unmarshaling cache data: %v", err)
			cs = cacheErrorStatus
		} else {
			// Restore headers from cache
			for key, vals := range data.Headers {
				for _, val := range vals {
					w.Header().Add(key, val)
				}
			}
			if m.cfg.AddStatusHeader {
				w.Header().Set(cacheHeader, cacheHitStatus)
			}
			w.WriteHeader(data.Status)
			if _, err := w.Write(data.Body); err != nil {
				log.Printf("Error writing cached response body: %v", err)
			}
			return
		}
	}

	if m.cfg.AddStatusHeader {
		w.Header().Set(cacheHeader, cs)
	}

	rw := &responseWriter{ResponseWriter: w}
	m.next.ServeHTTP(rw, r)

	expiry, ok := m.cacheable(r, w, rw.status)
	if !ok {
		return
	}

	data := cacheData{
		Status:  rw.status,
		Headers: w.Header(),
		Body:    rw.body,
	}

	b, err = json.Marshal(data)
	if err != nil {
		log.Printf("Error serializing cache item: %v", err)
		return
	}

	if err = m.cache.Set(key, b, expiry); err != nil {
		log.Printf("Error setting cache item: %v", err)
	}
}

func (m *cache) cacheable(r *http.Request, w http.ResponseWriter, status int) (time.Duration, bool) {
	// Don't cache error responses
	if status < 200 || status >= 400 {
		return 0, false
	}

	reasons, expireBy, err := cachecontrol.CachableResponseWriter(r, status, w, cachecontrol.Options{})
	if err != nil {
		log.Printf("Error determining cacheability: %v", err)
		return 0, false
	}

	if len(reasons) > 0 {
		// Debugging: log reasons why not cacheable
		if log.Flags() != 0 { // Only log if logging is enabled
			log.Printf("Response not cacheable for %s: %v", r.URL.Path, reasons)
		}
		return 0, false
	}

	expiry := time.Until(expireBy)
	if expiry <= 0 {
		return 0, false
	}

	maxExpiry := time.Duration(m.cfg.MaxExpiry) * time.Second
	if maxExpiry < expiry {
		expiry = maxExpiry
	}

	return expiry, true
}

func cacheKey(r *http.Request) string {
	// Base key with method, host and path
	key := r.Method + r.Host + r.URL.Path

	// Handle query parameters in a sorted, consistent way
	if len(r.URL.Query()) > 0 {
		// Get all query parameter keys
		params := make([]string, 0, len(r.URL.Query()))
		for param := range r.URL.Query() {
			params = append(params, param)
		}

		// Sort the parameter keys
		sort.Strings(params)

		var queryParts []string
		for _, param := range params {
			values := r.URL.Query()[param]
			sort.Strings(values)

			for _, value := range values {
				queryParts = append(queryParts, url.QueryEscape(param)+"="+url.QueryEscape(value))
			}
		}

		// Join all parameters with &
		key += "?" + strings.Join(queryParts, "&")
	}

	return key
}

type responseWriter struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (rw *responseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}

func (rw *responseWriter) Write(p []byte) (int, error) {
	rw.body = append(rw.body, p...)
	return rw.ResponseWriter.Write(p)
}

func (rw *responseWriter) WriteHeader(s int) {
	rw.status = s
	rw.ResponseWriter.WriteHeader(s)
}
