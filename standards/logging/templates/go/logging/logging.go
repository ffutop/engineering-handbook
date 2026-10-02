// Package logging 提供全工程统一的结构化日志（log/slog）。
//
// 字段约定：msg 为固定文案（不插入变量），event 为稳定的英文事件码，
// 其余上下文以独立字段输出。完整规范见 engineering-handbook「Go 结构化日志标准」。
package logging

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Options 是日志输出配置，对应配置文件中的 log 段
type Options struct {
	Level  string // debug | info | warn | error，空值为 info
	Format string // line | json，空值为 line（混合行：人可读的前缀列 + JSON 上下文，见 lineHandler）
}

// Validate 校验日志等级与格式
func (o Options) Validate() error {
	var errs []error
	if _, err := parseLevel(o.Level); err != nil {
		errs = append(errs, err)
	}
	if _, err := parseFormat(o.Format); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// LogValue 输出生效的等级与格式（空值按默认值展示）
func (o Options) LogValue() slog.Value {
	level, _ := parseLevel(o.Level)
	format, _ := parseFormat(o.Format)
	return slog.GroupValue(slog.String("level", strings.ToLower(level.String())), slog.String("format", format))
}

// New 按配置创建 logger，配置无效时返回错误而不是静默回退
func New(w io.Writer, o Options) (*slog.Logger, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	level, _ := parseLevel(o.Level)
	format, _ := parseFormat(o.Format)

	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	} else {
		h = newLineHandler(w, level)
	}
	return slog.New(h), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	return 0, fmt.Errorf("日志等级 %q 无效，可选 debug、info、warn、error", s)
}

func parseFormat(s string) (string, error) {
	switch strings.ToLower(s) {
	case "", "line":
		return "line", nil
	case "json":
		return "json", nil
	}
	return "", fmt.Errorf("日志格式 %q 无效，可选 line、json", s)
}
