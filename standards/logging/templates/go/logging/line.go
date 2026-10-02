package logging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"unicode"
)

// lineHandler 输出"固定前缀列 + JSON 上下文"的混合行，一条日志恒为一行：
//
//	2026-10-02T13:54:20.846+08:00 INFO  request.handled      请求处理完成 | {"conn":"127.0.0.1:64331","tenant":"t1"}
//
// 前缀列（时间、等级、event、msg）供人阅读；上下文由内部的 slog.JSONHandler 编码，
// 转义、LogValuer、group 等语义与 json 格式完全一致。
// 采集端用 dissect 切分前缀（%{ts} %{level->} %{event->} %{msg} | %{ctx}），再用 decode_json_fields 解析 ctx。
type lineHandler struct {
	out     *lineOutput  // 派生出的全部 handler 共享
	inner   slog.Handler // 只编码上下文字段，写入 out.ctx
	event   string       // 通过 With 绑定的 event
	grouped bool         // 已进入 group，此后的 event 属于 group 内字段，不再提取到前缀
}

// lineOutput 是共享的输出端：inner 编码与整行写出在同一把锁内完成
type lineOutput struct {
	mu  sync.Mutex
	w   io.Writer
	ctx bytes.Buffer // inner 的输出缓冲，每条日志重置
}

const (
	eventKey   = "event"
	eventWidth = 20 // event 列补齐宽度，超长时只隔一个空格
	timeLayout = "2006-01-02T15:04:05.000Z07:00"
)

// msgReplacer 保证 msg 不破坏行结构：换行转义；"|" 一律替换，否则 msg 以 " |" 结尾时会与分隔符相连而切分错位
var msgReplacer = strings.NewReplacer("\n", `\n`, "\r", `\r`, "|", "¦")

func newLineHandler(w io.Writer, level slog.Leveler) *lineHandler {
	out := &lineOutput{w: w}
	inner := slog.NewJSONHandler(&out.ctx, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 {
				switch a.Key {
				case slog.TimeKey, slog.LevelKey, slog.MessageKey:
					return slog.Attr{} // 已输出到前缀列
				}
			}
			return a
		},
	})
	return &lineHandler{out: out, inner: inner}
}

func (h *lineHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *lineHandler) Handle(ctx context.Context, r slog.Record) error {
	event := h.event
	rec := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == eventKey && !h.grouped {
			event = a.Value.String() // 调用时传入的 event 优先于 With 绑定的
			return true
		}
		rec.AddAttrs(a)
		return true
	})

	h.out.mu.Lock()
	defer h.out.mu.Unlock()
	h.out.ctx.Reset()
	if err := h.inner.Handle(ctx, rec); err != nil {
		return err
	}

	var line bytes.Buffer
	line.WriteString(r.Time.Format(timeLayout))
	fmt.Fprintf(&line, " %-5s %-*s %s | ", r.Level.String(), eventWidth, column(eventColumn(event)), column(msgReplacer.Replace(r.Message)))
	line.Write(bytes.TrimSuffix(h.out.ctx.Bytes(), []byte("\n")))
	line.WriteByte('\n')
	_, err := h.out.w.Write(line.Bytes())
	return err
}

func (h *lineHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := *h
	rest := make([]slog.Attr, 0, len(attrs))
	for _, a := range attrs {
		if a.Key == eventKey && !h.grouped {
			nh.event = a.Value.String()
			continue
		}
		rest = append(rest, a)
	}
	nh.inner = h.inner.WithAttrs(rest)
	return &nh
}

func (h *lineHandler) WithGroup(name string) slog.Handler {
	nh := *h
	nh.inner = h.inner.WithGroup(name)
	nh.grouped = true
	return &nh
}

// eventColumn 将 event 中的空白与控制字符替换为 _，保证其为单个不换行的列
func eventColumn(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return '_'
		}
		return r
	}, s)
}

// column 保证前缀列非空，避免采集端切分错位
func column(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
