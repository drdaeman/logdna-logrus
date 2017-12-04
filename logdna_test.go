package logdna

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/gogap/logrus_mate"
	"github.com/sirupsen/logrus"
)

type TestConfig struct {
	URL string
}

func (tcfg *TestConfig) GetBoolean(path string, defaultVal ...bool) bool {
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return false
}
func (tcfg *TestConfig) GetByteSize(path string) *big.Int {
	return big.NewInt(0)
}
func (tcfg *TestConfig) GetInt32(path string, defaultVal ...int32) int32 {
	if path == "size" {
		return 4
	}
	if path == "qsize" {
		return 2
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0
}
func (tcfg *TestConfig) GetInt64(path string, defaultVal ...int64) int64 {
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0
}
func (tcfg *TestConfig) GetString(path string, defaultVal ...string) string {
	if path == "api-key" {
		return "test-api-key"
	}
	if path == "url" {
		return tcfg.URL
	}
	if path == "app" {
		return "testapp"
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}
func (tcfg *TestConfig) GetFloat32(path string, defaultVal ...float32) float32 {
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0

}
func (tcfg *TestConfig) GetFloat64(path string, defaultVal ...float64) float64 {
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0

}
func (tcfg *TestConfig) GetTimeDuration(path string, defaultVal ...time.Duration) time.Duration {
	if path == "flush" {
		return time.Duration(1 * time.Second)
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return time.Duration(0)
}
func (tcfg *TestConfig) GetTimeDurationInfiniteNotAllowed(path string, defaultVal ...time.Duration) time.Duration {
	return tcfg.GetTimeDuration(path, defaultVal...)
}
func (tcfg *TestConfig) GetBooleanList(path string) []bool {
	return []bool{}
}
func (tcfg *TestConfig) GetFloat32List(path string) []float32 {
	return []float32{}
}
func (tcfg *TestConfig) GetFloat64List(path string) []float64 {
	return []float64{}
}
func (tcfg *TestConfig) GetInt32List(path string) []int32 {
	return []int32{}
}
func (tcfg *TestConfig) GetInt64List(path string) []int64 {
	return []int64{}
}
func (tcfg *TestConfig) GetByteList(path string) []byte {
	return []byte{}
}
func (tcfg *TestConfig) GetStringList(path string) []string {
	return []string{}
}
func (tcfg *TestConfig) GetConfig(path string) logrus_mate.Configuration {
	return tcfg
}
func (tcfg *TestConfig) WithFallback(fallback logrus_mate.Configuration) {
	// Nothing to do here
}
func (tcfg *TestConfig) HasPath(path string) bool {
	return false
}
func (tcfg *TestConfig) Keys() []string {
	return []string{}
}

func checkLines(t *testing.T, body []byte, expect []string, expectMeta []map[string]interface{}) {
	var data map[string]interface{}

	if err := json.Unmarshal(body, &data); err != nil {
		t.Error(err)
		return
	}

	lines, ok := data["lines"]
	if !ok {
		t.Error("POSTed body has no 'lines' key")
		return
	}

	switch v := lines.(type) {
	case []interface{}:
		if len(v) != len(expect) {
			t.Error("POSTed body has unexpected number of lines")
			return
		}
		t.Logf("Success: POSTed body has exactly %d line(s)", len(v))

		for idx := range v {
			line := v[idx]

			expected := expect[idx]
			var expectedMeta map[string]interface{}
			if idx < len(expectMeta) {
				expectedMeta = expectMeta[idx]
			} else {
				expectedMeta = nil
			}

			switch lv := line.(type) {
			case map[string]interface{}:
				text, ok := lv["line"]
				if !ok {
					t.Errorf("Line %d has no 'line' key", idx+1)
					continue
				}
				if text != expected {
					t.Errorf("Line %d has unexpected text", idx+1)
					continue
				}
				t.Logf("Success: Line %d matches the expected text", idx+1)

				if lv["app"] != "testapp" {
					t.Errorf("Line %d has no or wrong 'app' value", idx+1)
				}

				if expectedMeta != nil {
					meta, ok := lv["meta"]
					if len(expectedMeta) > 0 && !ok {
						t.Errorf("Line %d has no 'meta' key while it was expected here", idx+1)
						continue
					} else if len(expectedMeta) == 0 && !ok {
						t.Logf("Success: Line %d doesn't have meta", idx+1)
					} else {
						switch mv := meta.(type) {
						case map[string]interface{}:
							if !reflect.DeepEqual(mv, expectedMeta) {
								t.Errorf("Line %d has unexpected meta", idx+1)
								continue
							}
							t.Logf("Success: Line %d matches the expected meta", idx+1)
						default:
							t.Errorf("Line %d has wrong type for meta: %v", idx+1, reflect.TypeOf(meta))
						}
					}
				}
			default:
				t.Error("Line %d has incorrect type %v", idx+1, reflect.TypeOf(line))
				continue
			}
		}
	default:
		t.Errorf("POSTed body has incorrect type %v", reflect.TypeOf(lines))
	}
}

func TestLogDNA(t *testing.T) {
	c := make(chan []byte, 32) // Must be large enough to receive all the bodies and not block

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || !(username == "test-api-key" || password == "test-api-key") {
			t.Error("Request is missing or has invalid invalid auth header")
		}
		query := r.URL.Query()
		if query.Get("hostname") == "" {
			t.Error("Request is missing 'hostname' query argument")
		}
		if query.Get("apikey") != "" {
			// API key should be passed in the request headers (only!)
			t.Error("Request has 'apikey' query argument (it shouldn't)")
		}
		ts := query.Get("now")
		if ts == "" {
			t.Error("Request is missing 'now' query argument")
		} else {
			timestamp, err := strconv.ParseInt(ts, 10, 64)
			if err != nil {
				t.Error("Failed to parse 'now' query argument")
			} else {
				nowMilliseconds := time.Now().UnixNano() / 1000000
				if timestamp > nowMilliseconds {
					t.Error("Provided time is in the future (clock went backwards?)")
				} else if nowMilliseconds-timestamp > 1000 {
					// Normally, it should be nearly instant
					t.Error("Provided time is too far in the past (very slow host?)")
				}
			}
		}
		body, _ := ioutil.ReadAll(r.Body)
		c <- body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer ts.Close()

	// TODO: Test LineJSON mode
	hook, err := logrus_mate.NewHook("logdna", &TestConfig{URL: ts.URL})
	if err != nil {
		t.Error(err)
		return
	}
	if len(hook.Levels()) < 1 {
		t.Error("Hook levels are empty")
	}
	logger := logrus.New()
	logger.Out = ioutil.Discard
	// In newer (yet untagged Logrus versions) use logger.AddHook(hook)
	logger.Hooks.Add(hook)

	// Test that logger flushes on timeout
	logger.Info("Test log entry")
	select {
	case body := <-c:
		checkLines(t, body, []string{"Test log entry"}, []map[string]interface{}{})
	case <-time.After(2 * time.Second):
		t.Error("Timed out")
	}

	// Test structured logging
	logger.WithFields(logrus.Fields{
		"Test1": "Value1",
		"Test2": "Value2",
	}).Info("Test log entry, with data")
	select {
	case body := <-c:
		checkLines(
			t, body, []string{"Test log entry, with data"},
			[]map[string]interface{}{{"Test1": "Value1", "Test2": "Value2"}},
		)
	case <-time.After(2 * time.Second):
		t.Error("Timed out")
	}

	// Test that logger doesn't make requests when there's nothing to do
	select {
	case <-c:
		t.Error("Had flushed something when nothing was logged")
	case <-time.After(2 * time.Second):
		t.Log("Success: Received no log entries when none was sent")
	}

	// Test that logger flushes on buffer being full
	for i := 1; i <= 7; i++ { // size + qsize + 1 ensures blocking to flush
		if i == 2 {
			logger.WithField("Foo", "Bar").Info("Test log entry 2")
		} else {
			logger.Info(fmt.Sprintf("Test log entry %d", i))
		}
	}
	select {
	case body := <-c:
		checkLines(t, body, []string{
			"Test log entry 1",
			"Test log entry 2",
			"Test log entry 3",
			"Test log entry 4",
		}, []map[string]interface{}{
			map[string]interface{}{},
			map[string]interface{}{"Foo": "Bar"},
			map[string]interface{}{},
			map[string]interface{}{},
		})
	default:
		t.Error("Logged had not flushed")
	}
}
