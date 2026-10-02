package logtest

import "testing"

// 期望值的 Go 类型与 JSON 解码后的类型不同时仍可比较
func TestExpectFieldTypes(t *testing.T) {
	l, rec := New(t)
	l.Info("请求处理完成", "event", "request.handled", "n", int32(3), "ratio", float32(1.5), "ok", true, "tags", []string{"a"}, "m", map[string]int{"k": 1})
	rec.Expect(t, "request.handled", "INFO", map[string]any{"n": int32(3), "ratio": float32(1.5), "ok": true, "tags": []string{"a"}, "m": map[string]int{"k": 1}})
	rec.ExpectNone(t, "request.invalid")
}
