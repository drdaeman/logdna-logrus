package logdna

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogap/logrus_mate"
	"github.com/sirupsen/logrus"
)

// DefaultIngestURL is the URL of the LogDNA ingestion API endpoint
const DefaultIngestURL = "https://logs.logdna.com/logs/ingest"

// Config is the configuration struct for LogDNAHook
type Config struct {
	IngestURL string
	APIKey    string
	Hostname  string
	MAC       string
	IP        string
	App       string // NOTE: App and Env are global at the moment
	Env       string
}

// Hook is a Logrus hook that sends entries to LogDNA
type Hook struct {
	Config     *Config
	BufferSize int
	FlushEvery time.Duration
	MayDrop    bool
	LineJSON   bool
	c          chan *logEntry
	wg         *sync.WaitGroup
}

type logEntry struct {
	Timestamp int64                  `json:"timestamp"`
	Line      string                 `json:"line"`
	Level     string                 `json:"level,omitempty"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
	config    *Config
}

// MarshalJSON marshals logEntry to JSON
func (entry *logEntry) MarshalJSON() ([]byte, error) {
	type LogEntry logEntry
	return json.Marshal(&struct {
		App string `json:"app,omitempty"`
		Env string `json:"env,omitempty"`
		*LogEntry
	}{
		App:      entry.config.App,
		Env:      entry.config.Env,
		LogEntry: (*LogEntry)(entry),
	})
}

// Levels returns a list of all supported log levels
func (hook *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}

// Fire queues the message for submission to LogDNA
func (hook *Hook) Fire(entry *logrus.Entry) error {
	e := &logEntry{
		Timestamp: entry.Time.UnixNano() / 1000000,
		Level:     strings.ToUpper(entry.Level.String()),
		config:    hook.Config,
	}
	if hook.LineJSON {
		meta := make(map[string]interface{})
		for k, v := range entry.Data {
			if k == "message" {
				// Don't use both "message" and "_message" in your log metadata
				k = "_message"
			}
			meta[k] = v
		}
		meta["message"] = entry.Message
		line, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		e.Line = string(line)
		e.Meta = nil
	} else {
		meta := entry.Data
		if len(meta) == 0 {
			// Don't bother sending empty maps
			meta = nil
		}
		e.Line = entry.Message
		e.Meta = meta
	}
	if hook.MayDrop {
		// May discard the entry if channel's full, but won't block.
		select {
		case hook.c <- e:
		default:
		}
	} else {
		// Won't lose data but may block if channel's full.
		hook.c <- e
	}
	return nil
}

// Flush sends all the pending entries to the LogDNA ingestion API
func (hook *Hook) Flush() error {
	hook.c <- nil
	return nil
}

// Close stops the processing
func (hook *Hook) Close() error {
	close(hook.c)
	hook.wg.Wait()
	return nil
}

func (config *Config) flush(buffer *[]*logEntry) error {
	if buffer == nil || len(*buffer) == 0 {
		return nil
	}

	body, err := json.Marshal(&struct {
		Lines []*logEntry `json:"lines"`
	}{
		Lines: *buffer,
	})
	if err != nil {
		return err
	}

	client := &http.Client{} // TODO: Make this user-configurable?
	bodyReader := bytes.NewReader(body)
	req, err := http.NewRequest("POST", config.IngestURL, bodyReader)
	req.SetBasicAuth("", config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	q := req.URL.Query()
	q.Add("hostname", config.Hostname)
	q.Add("now", fmt.Sprintf("%d", time.Now().UnixNano()/1000000))
	if config.MAC != "" {
		q.Add("mac", config.MAC)
	}
	if config.IP != "" {
		q.Add("ip", config.IP)
	}
	req.URL.RawQuery = q.Encode()
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode > 299 || response.StatusCode < 200 {
		return fmt.Errorf("HTTP error %d", response.StatusCode)
	}
	if response.StatusCode == 204 {
		// Just in case LogDNA will decide to respond with HTTP 204
		return nil
	}
	var res map[string]interface{}
	b, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &res)
	if err != nil {
		return err
	}
	status, ok := res["status"]
	if !ok {
		return errors.New("LogDNA response has no 'status' key")
	}
	if !ok || status != "ok" {
		return fmt.Errorf("LogDNA responded with non-OK status: %v", status)
	}
	return nil
}

func (hook *Hook) run() chan *logEntry {
	hook.wg.Add(1) // TODO: Consider only locking when buffer has something?
	defer hook.wg.Done()

	buffer := make([]*logEntry, 0)
	timeout := time.After(hook.FlushEvery)
	for {
		select {
		case entry, ok := <-hook.c:
			if !ok || entry == nil {
				err := hook.Config.flush(&buffer)
				if err == nil {
					buffer = make([]*logEntry, 0)
					if !ok {
						hook.c = nil
						return nil
					}
				} else if !ok {
					// We were requested to terminate but can't
					panic(fmt.Sprintf("Failed to flush to LogDNA: %v", err))
				}
			}
			buffer = append(buffer, entry)
			if len(buffer) >= hook.BufferSize {
				err := hook.Config.flush(&buffer)
				if err == nil || hook.MayDrop {
					// TODO: Consider retrying a few times?
					buffer = make([]*logEntry, 0)
					timeout = time.After(hook.FlushEvery)
				}
				// TODO: Wish there'd be some way to signal about the error
			}
		case <-timeout:
			if len(buffer) > 0 {
				err := hook.Config.flush(&buffer)
				if err == nil {
					buffer = make([]*logEntry, 0)
					timeout = time.After(hook.FlushEvery)
				}
			}
		}
	}
}

func init() {
	logrus_mate.RegisterHook("logdna", NewFromConfig)
}

// NewFromConfig creates a new LogDNA hook from the Logrus Mate configuration
func NewFromConfig(config logrus_mate.Configuration) (logrus.Hook, error) {
	apiKey := config.GetString("api-key")
	if apiKey == "" {
		return nil, errors.New("LogDNA API key is required")
	}

	hostname := config.GetString("hostname")
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			return nil, err
		}
	}

	bufferSize := int(config.GetInt32("size", 4096))
	hook := &Hook{
		Config: &Config{
			IngestURL: config.GetString("url", DefaultIngestURL),
			APIKey:    apiKey,
			Hostname:  hostname,
			MAC:       config.GetString("mac"),
			IP:        config.GetString("ip"),
			App:       config.GetString("app"),
			Env:       config.GetString("env"),
		},
		BufferSize: bufferSize,
		FlushEvery: config.GetTimeDuration("flush", 10*time.Second),
		MayDrop:    config.GetBoolean("drop", false),
		LineJSON:   config.GetBoolean("json", false),
		c:          make(chan *logEntry, int(config.GetInt32("qsize", 128))),
		wg:         &sync.WaitGroup{},
	}
	go hook.run()
	logrus.RegisterExitHandler(func() {
		hook.Close()
	})
	return hook, nil
}
