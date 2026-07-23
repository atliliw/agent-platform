package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestLoggerOutputsStatusAsNumber locks the fix where the Logger middleware
// rendered the HTTP status code via string(rune(status)) (a Unicode code point)
// instead of strconv.Itoa(status). For status 200 the buggy code printed "È"
// (U+00C8); the fix must print "200".
func TestLoggerOutputsStatusAsNumber(t *testing.T) {
	// Arrange
	originalWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = originalWriter }()

	router := gin.New()
	router.Use(Logger())
	router.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()

	// Act
	router.ServeHTTP(w, req)

	// Assert
	out := buf.String()
	if !strings.Contains(out, "200") {
		t.Errorf("expected log line to contain status \"200\", got %q", out)
	}
	// rune(200) == 'È' (U+00C8); its presence means the status was rendered as
	// a Unicode code point rather than a decimal string.
	if strings.Contains(out, "È") {
		t.Errorf("status rendered as Unicode rune, want number string: %q", out)
	}
}

// TestLoggerOutputsNonAsciiStatusAsNumber checks a status whose rune is an
// obscure character (404 -> U+0194) to confirm the decimal form is used.
func TestLoggerOutputsNonAsciiStatusAsNumber(t *testing.T) {
	// Arrange
	originalWriter := gin.DefaultWriter
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = originalWriter }()

	router := gin.New()
	router.Use(Logger())
	router.GET("/missing", func(c *gin.Context) { c.Status(http.StatusNotFound) })

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()

	// Act
	router.ServeHTTP(w, req)

	// Assert
	out := buf.String()
	if !strings.Contains(out, "404") {
		t.Errorf("expected log line to contain status \"404\", got %q", out)
	}
}
