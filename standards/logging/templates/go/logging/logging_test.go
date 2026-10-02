package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewJSON(t *testing.T) {
	var buf bytes.Buffer
	l, err := New(&buf, Options{Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	l.Debug("不应输出", "event", "x.debug")
	l.Info("连接已建立", "event", "conn.opened", "conn", "1.2.3.4:5")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("默认等级为 info，期望 1 行日志，实际 %d 行：%s", len(lines), buf.String())
	}
	var e map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &e); err != nil {
		t.Fatalf("json 格式应输出合法的 JSON：%v", err)
	}
	for k, want := range map[string]any{"level": "INFO", "msg": "连接已建立", "event": "conn.opened", "conn": "1.2.3.4:5"} {
		if e[k] != want {
			t.Errorf("字段 %s 期望 %v，实际 %v", k, want, e[k])
		}
	}
	for _, k := range []string{"service", "version", "component"} {
		if _, ok := e[k]; ok {
			t.Errorf("每行日志不应附带 %s 字段", k)
		}
	}
}

func TestNewLevel(t *testing.T) {
	var buf bytes.Buffer
	l, err := New(&buf, Options{Level: "DEBUG"})
	if err != nil {
		t.Fatal(err)
	}
	l.Debug("收到原始数据", "event", "input.received")
	if out := buf.String(); !strings.Contains(out, " DEBUG input.received ") {
		t.Fatalf("期望输出 DEBUG 日志，实际：%q", out)
	}
}

func TestOptionsValidate(t *testing.T) {
	err := Options{Level: "trace", Format: "text"}.Validate()
	if err == nil {
		t.Fatal("期望校验失败")
	}
	for _, want := range []string{`日志等级 "trace" 无效`, `日志格式 "text" 无效`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息应包含 %q，实际：%v", want, err)
		}
	}
	if _, err := New(&bytes.Buffer{}, Options{Level: "trace"}); err == nil {
		t.Error("无效配置不应创建 logger")
	}
}

func TestOptionsLogValue(t *testing.T) {
	var buf bytes.Buffer
	l, _ := New(&buf, Options{})
	l.Info("配置已加载", "log", Options{Level: "WARN"})
	if out := buf.String(); !strings.Contains(out, `"log":{"level":"warn","format":"line"}`) {
		t.Fatalf("期望输出生效的日志配置，实际：%s", out)
	}
}
