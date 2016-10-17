package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/spf13/cobra"
)

var (
	hostname string
)

var l = log.New(os.Stderr, "", 0)

func main() {
	c := &cobra.Command{Use: "hello", Short: "Run hello server"}
	c.AddCommand(&cobra.Command{Use: "reset", RunE: reset, Short: "Reset counter"})
	c.RunE = server
	err := c.Execute()
	if err != nil {
		os.Exit(1)
	}
}

const redisCounterKey = "requests"

func reset(cmd *cobra.Command, args []string) error {
	c, err := connectRedis()
	if err != nil {
		return err
	}
	defer c.Close()
	c.Send("multi")
	c.Send("GET", redisCounterKey)
	c.Send("DEL", redisCounterKey)
	v, err := redis.Values(c.Do("EXEC"))
	if err != nil {
		return err
	}
	if len(v) != 2 {
		return fmt.Errorf("expected length 2, was %d", len(v))
	}
	var i int
	if v[0] != nil {
		i, err = redis.Int(v[0], nil)
		if err != nil {
			return fmt.Errorf("error casting: %s", err)
		}
		l.Printf("requests was %d", i)
	} else {
		l.Printf("key did not exist")
	}
	return nil
}

func server(cmd *cobra.Command, args []string) (err error) {
	hostname, err = os.Hostname()
	if err != nil {
		l.Fatal(err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	if !strings.Contains(port, ":") {
		port = "0.0.0.0:" + port
	}
	l.Printf("running address %q", port)

	b, err := ioutil.ReadFile("/etc/version")
	if err != nil {
		l.Fatal(err)
	}
	version := strings.TrimSpace(string(b))
	return http.ListenAndServe(port, mux(l, version))
}

func mux(l Logger, version string) *http.ServeMux {
	m := http.NewServeMux()
	handler, ok := actions[version]
	if !ok {
		handler = current
	}
	m.HandleFunc("/_status", statusHandler(version))
	m.HandleFunc("/", handle(l, version, handler(version)))
	for k, v := range actions {
		m.HandleFunc("/"+k, handle(l, version, v(version)))
	}
	return m
}

var actions = map[string]handler{
	"v1": v1,
	"v2": v2,
	"v3": v3,
}

const defaultMessage = "Hello World!"

func statusHandler(version string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := func() error {
			start := time.Now()
			c, err := connectRedis()
			if err != nil {
				return err
			}
			defer c.Close()
			rsp := struct {
				Status    bool    `json:"status,omitempty"`
				Took      float64 `json:"took,omitempty"`
				Version   string  `json:"version,omitempty"`
				Hostname  string  `json:"hostname,omitempty"`
				GoVersion string  `json:"go_version,omitempty"`
			}{
				Version:   version,
				Hostname:  hostname,
				GoVersion: runtime.Version(),
			}
			s, err := redis.String(c.Do("PING"))
			if err != nil {
				return err
			} else if s != "PONG" {
				return fmt.Errorf("expected %q, was %q", "PONG", s)
			}
			rsp.Status = true
			rsp.Took = time.Since(start).Seconds()
			return renderJSON(w, rsp)
		}()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), 500)
		}
	}
}

var current = v3

// hardcoded
func v1(version string) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		return renderJSON(w, newResponse(version))
	}
}

// with env
func v2(version string) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		rsp := newResponseWithENVMessage(version)
		err := renderJSON(w, rsp)
		if err != nil {
			return err
		}
		return nil
	}
}

// with redis
func v3(version string) func(w http.ResponseWriter, r *http.Request) error {
	return func(w http.ResponseWriter, r *http.Request) error {
		c := pool.Get()
		defer c.Close()
		s, err := redis.String(c.Do("INFO"))
		if err != nil {
			return err
		}
		rsp := newResponseWithENVMessage(version)
		rsp.RedisVersion, _ = extractVersion(s)

		rsp.Requests, err = redis.Int(c.Do("INCR", redisCounterKey))
		if err != nil {
			return err
		}

		err = renderJSON(w, rsp)
		if err != nil {
			return err
		}

		return nil
	}
}

type Logger interface {
	Printf(string, ...interface{})
}

var newLogger = func() Logger {
	return log.New(os.Stderr, "", 0)
}

func handle(l Logger, version string, f func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		err := f(w, r)
		msg := fmt.Sprintf("addr=%s method=%s url=%s host=%s ua=%q total_time=%.06f", r.RemoteAddr, r.Method, r.URL.String(), r.Host, r.UserAgent(), time.Since(start).Seconds())
		if err != nil {
			r := newResponse(version)
			r.Error = err.Error()
			msg += fmt.Sprintf(" err=%q", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			_ = renderJSON(w, r)
		}
		l.Printf("%s", msg)
	}
}

var pool = redis.NewPool(connectRedis, 1)

func connectRedis() (redis.Conn, error) {
	raw := os.Getenv("REDIS_URL")
	if raw == "" {
		return nil, fmt.Errorf("REDIS_URL must be present")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":6379"
	}
	c, err := redis.Dial("tcp", host)
	if err != nil {
		return nil, fmt.Errorf("error connecting host=%q: %s", host, err)
	}
	return c, nil
}

func renderJSON(w http.ResponseWriter, i interface{}) error {
	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(i)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, buf)
	return nil
}

type handler func(string) func(w http.ResponseWriter, r *http.Request) error

func extractVersion(info string) (string, bool) {
	for _, p := range strings.Split(info, "\r\n") {
		parts := strings.SplitN(p, ":", 2)
		if len(parts) == 2 && parts[0] == "redis_version" {
			return parts[1], true
		}
	}
	return "", false
}

type response struct {
	Error        string `json:"error,omitempty"`
	Host         string `json:"host,omitempty"`
	Message      string `json:"message,omitempty"`
	RedisVersion string `json:"redis_info,omitempty"`
	Requests     int    `json:"requests,omitempty"`
	Version      string `json:"version,omitempty"`
}

func newResponse(version string) *response {
	return &response{Host: hostname, Version: version, Message: defaultMessage}
}

func newResponseWithENVMessage(version string) *response {
	r := newResponse(version)
	r.Message = defaultMessage
	if msg := os.Getenv("MESSAGE"); msg != "" {
		r.Message = msg
	}
	return r
}
