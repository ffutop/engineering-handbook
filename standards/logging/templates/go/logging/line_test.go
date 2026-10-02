package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 10, 2, 13, 54, 20, 846_000_000, time.FixedZone("CST", 8*3600))

// handle 以固定时间输出一条日志，返回输出内容
func handle(t *testing.T, h slog.Handler, level slog.Level, msg string, attrs ...slog.Attr) string {
	t.Helper()
	r := slog.NewRecord(fixedTime, level, msg, 0)
	r.AddAttrs(attrs...)
	var buf bytes.Buffer
	h.(*lineHandler).out.w = &buf
	if err := h.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// dissected 是按采集端 dissect 规则 `%{ts} %{level->} %{event->} %{msg} | %{ctx}` 切分的结果
type dissected struct {
	ts, level, event, msg string
	ctx                   map[string]any
}

// dissect 在 Go 中复现采集端的切分规则：以空格切出 ts，连续空格视为一个分隔（-> 修饰），
// msg 截止到第一个 " | "，其后整体为 ctx JSON
func dissect(t *testing.T, line string) dissected {
	t.Helper()
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("一条日志必须恰好一行：%q", line)
	}
	rest := strings.TrimSuffix(line, "\n")
	next := func() string {
		i := strings.IndexByte(rest, ' ')
		if i < 0 {
			t.Fatalf("前缀列不完整：%q", line)
		}
		field := rest[:i]
		rest = strings.TrimLeft(rest[i:], " ")
		return field
	}
	var d dissected
	d.ts, d.level, d.event = next(), next(), next()
	i := strings.Index(rest, " | ")
	if i < 0 {
		t.Fatalf("缺少分隔符 \" | \"：%q", line)
	}
	d.msg = rest[:i]
	if err := json.Unmarshal([]byte(rest[i+3:]), &d.ctx); err != nil {
		t.Fatalf("ctx 不是合法的 JSON：%v\n%q", err, line)
	}
	if _, err := time.Parse(time.RFC3339, d.ts); err != nil {
		t.Fatalf("时间列无法解析：%v", err)
	}
	return d
}

func TestLineFormat(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	got := handle(t, h, slog.LevelInfo, "请求处理完成",
		slog.String("event", "request.handled"), slog.String("tenant", "t1"), slog.Int("records", 5), slog.Bool("published", true))
	want := "2026-10-02T13:54:20.846+08:00 INFO  request.handled      请求处理完成 | {\"tenant\":\"t1\",\"records\":5,\"published\":true}\n"
	if got != want {
		t.Fatalf("格式不符\n期望 %q\n实际 %q", want, got)
	}
}

// 没有上下文字段时 ctx 为 {}；缺少 event 或 msg 时以 - 占位，保证列数不变
func TestLineEmptyColumns(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	got := handle(t, h, slog.LevelWarn, "")
	if want := "2026-10-02T13:54:20.846+08:00 WARN  -                    - | {}\n"; got != want {
		t.Fatalf("期望 %q\n实际 %q", want, got)
	}
	dissect(t, got)
}

// 超长 event 不截断，只隔一个空格
func TestLineLongEvent(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	d := dissect(t, handle(t, h, slog.LevelWarn, "记录校验和不匹配，已跳过该条", slog.String("event", "protocol.record_checksum_mismatch")))
	if d.event != "protocol.record_checksum_mismatch" || d.msg != "记录校验和不匹配，已跳过该条" {
		t.Fatalf("切分结果不符：%+v", d)
	}
}

// With 绑定的字段进入 ctx，With 绑定的 event 进入前缀列，调用时传入的 event 优先
func TestLineWithAttrs(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo).
		WithAttrs([]slog.Attr{slog.String("event", "lib.internal"), slog.String("conn", "1.2.3.4:5")}).
		WithAttrs([]slog.Attr{slog.String("tenant", "t1")})

	d := dissect(t, handle(t, h, slog.LevelError, "[client] error"))
	if d.event != "lib.internal" || d.ctx["conn"] != "1.2.3.4:5" || d.ctx["tenant"] != "t1" {
		t.Fatalf("切分结果不符：%+v", d)
	}
	if _, ok := d.ctx["event"]; ok {
		t.Error("event 不应出现在 ctx 中")
	}
	if d := dissect(t, handle(t, h, slog.LevelInfo, "x", slog.String("event", "conn.closed"))); d.event != "conn.closed" {
		t.Errorf("调用时传入的 event 应优先，实际 %s", d.event)
	}
}

// group 内的同名字段属于业务数据，不提取到前缀列
func TestLineGroup(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo).WithGroup("g")
	d := dissect(t, handle(t, h, slog.LevelInfo, "x", slog.String("event", "inner"), slog.Int("n", 1)))
	g, _ := d.ctx["g"].(map[string]any)
	if d.event != "-" || g["event"] != "inner" || g["n"] != float64(1) {
		t.Fatalf("group 处理不符：%+v", d)
	}
}

// msg、event 中的换行与分隔符不得破坏行结构；ctx 中的换行、引号由 JSON 转义
func TestLineEscaping(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	got := handle(t, h, slog.LevelError, "第一行\n第二行 | 尾部",
		slog.String("event", "worker.panic"),
		slog.String("stack", "goroutine 1:\nmain.main()"),
		slog.String("payload", `[{"k":"a | b"}]`),
		slog.Any("err", errors.New("listen tcp :8080: bind: address already in use")),
	)
	d := dissect(t, got)
	if d.msg != `第一行\n第二行 ¦ 尾部` {
		t.Errorf("msg 转义不符：%q", d.msg)
	}
	if d.ctx["stack"] != "goroutine 1:\nmain.main()" || d.ctx["payload"] != `[{"k":"a | b"}]` || d.ctx["err"] != "listen tcp :8080: bind: address already in use" {
		t.Errorf("ctx 内容不符：%+v", d.ctx)
	}
}

// msg 以 " |" 结尾时不得与分隔符相连；event 中的换行与空白替换为 _
func TestLineEscapingEdges(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	d := dissect(t, handle(t, h, slog.LevelInfo, "尾部 |", slog.String("event", "a b\nc"), slog.Int("n", 1)))
	if d.msg != "尾部 ¦" || d.event != "a_b_c" || d.ctx["n"] != float64(1) {
		t.Fatalf("切分结果不符：%+v", d)
	}
}

type secret struct{ user, password string }

func (s secret) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", s.user), slog.String("password", "***"))
}

// LogValuer 在 ctx 中按其 LogValue 输出（用于配置打码）
func TestLineLogValuer(t *testing.T) {
	h := newLineHandler(nil, slog.LevelInfo)
	got := handle(t, h, slog.LevelInfo, "配置已加载", slog.Any("db", secret{"u", "p@ss"}))
	if strings.Contains(got, "p@ss") || !strings.Contains(got, `"db":{"user":"u","password":"***"}`) {
		t.Fatalf("LogValuer 输出不符：%s", got)
	}
}

// 并发写入时每行完整，不交错
func TestLineConcurrent(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(newLineHandler(&buf, slog.LevelInfo))
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := base.With("conn_id", i)
			for j := range 50 {
				l.Info("请求处理完成", "event", "request.handled", "seq", j)
			}
		}()
	}
	wg.Wait()
	lines := strings.SplitAfter(buf.String(), "\n")
	lines = lines[:len(lines)-1]
	if len(lines) != 1000 {
		t.Fatalf("期望 1000 行，实际 %d 行", len(lines))
	}
	for _, line := range lines {
		if d := dissect(t, line); d.event != "request.handled" || d.ctx["conn_id"] == nil {
			t.Fatalf("行内容错乱：%q", line)
		}
	}
}

// 通过 New 创建的默认格式即为 line
func TestNewDefaultIsLine(t *testing.T) {
	var buf bytes.Buffer
	l, err := New(&buf, Options{})
	if err != nil {
		t.Fatal(err)
	}
	l.Info("连接已建立", "event", "conn.opened", "conn", "1.2.3.4:5")
	d := dissect(t, buf.String())
	if d.level != "INFO" || d.event != "conn.opened" || d.msg != "连接已建立" || d.ctx["conn"] != "1.2.3.4:5" {
		t.Fatalf("默认格式不符：%s", fmt.Sprint(d))
	}
}
