// Package logtest 为测试提供可断言的结构化日志：日志以 JSON 写入内存，按 event 与字段断言，
// 不依赖中文文案，修改 msg 不会破坏测试。
package logtest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"reflect"
	"sync"
	"testing"
)

// Entry 是一条解析后的日志
type Entry map[string]any

// Level 返回日志等级，如 "INFO"
func (e Entry) Level() string { s, _ := e["level"].(string); return s }

// Recorder 记录测试期间输出的全部日志，可在多个协程中并发写入
type Recorder struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (r *Recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.Write(p)
}

// New 返回 DEBUG 级别的 JSON logger 及其记录器
func New(t *testing.T) (*slog.Logger, *Recorder) {
	t.Helper()
	r := &Recorder{}
	return slog.New(slog.NewJSONHandler(r, &slog.HandlerOptions{Level: slog.LevelDebug})), r
}

// Entries 返回全部日志，可按 event 过滤（为空时返回全部）
func (r *Recorder) Entries(t *testing.T, event string) []Entry {
	t.Helper()
	r.mu.Lock()
	data := append([]byte(nil), r.buf.Bytes()...)
	r.mu.Unlock()

	var out []Entry
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("日志不是合法的 JSON：%v\n%s", err, sc.Bytes())
		}
		if event == "" || e["event"] == event {
			out = append(out, e)
		}
	}
	return out
}

// Expect 断言恰好有一条指定 event 的日志，校验其等级与字段，并返回该日志
func (r *Recorder) Expect(t *testing.T, event, level string, fields map[string]any) Entry {
	t.Helper()
	entries := r.Entries(t, event)
	if len(entries) != 1 {
		t.Fatalf("期望 1 条 event=%s 的日志，实际 %d 条\n全部日志：%s", event, len(entries), r.String())
	}
	e := entries[0]
	if e.Level() != level {
		t.Errorf("event=%s 等级期望 %s，实际 %s", event, level, e.Level())
	}
	for k, want := range fields {
		if got, ok := e[k]; !ok || !equal(got, want) {
			t.Errorf("event=%s 字段 %s 期望 %v，实际 %v（存在：%v）", event, k, want, got, ok)
		}
	}
	return e
}

// ExpectNone 断言不存在指定 event 的日志
func (r *Recorder) ExpectNone(t *testing.T, event string) {
	t.Helper()
	if entries := r.Entries(t, event); len(entries) != 0 {
		t.Fatalf("不应出现 event=%s 的日志，实际 %d 条：%v", event, len(entries), entries)
	}
}

func (r *Recorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.buf.String()
}

// equal 将期望值经 JSON 编解码后与日志中的值比较，数值、map、切片均按 JSON 语义比较
func equal(got, want any) bool {
	b, err := json.Marshal(want)
	if err != nil {
		return false
	}
	var w any
	if err := json.Unmarshal(b, &w); err != nil {
		return false
	}
	return reflect.DeepEqual(got, w)
}
