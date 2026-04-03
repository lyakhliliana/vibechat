package logger

import (
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/lumberjack.v2"
)

func Setup(cfg Config) {
	zerolog.TimeFieldFormat = time.RFC3339

	lvl, err := zerolog.ParseLevel(strings.ToLower(cfg.Level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	var w io.Writer
	switch cfg.Type {
	case TypeConsole:
		w = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05"}
	case TypeFile:
		w = newFileWriter(cfg.File)
	default:
		w = os.Stdout
	}

	log.Logger = zerolog.New(w).
		Level(lvl).
		With().
		Timestamp().
		Caller().
		Logger()
}

func newFileWriter(cfg *FileConfig) io.Writer {
	_ = os.MkdirAll(filepath.Dir(cfg.Path), 0755)

	// pre-create with desired permissions before lumberjack takes over
	if f, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.FileMode(cfg.Mode)); err == nil {
		_ = f.Close()
		_ = os.Chmod(cfg.Path, os.FileMode(cfg.Mode))
	}

	applyOwnership(cfg.Path, cfg.Owner, cfg.Group)

	return &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
	}
}

func applyOwnership(path, owner, group string) {
	if owner == "" && group == "" {
		return
	}

	uid, gid := -1, -1

	if owner != "" {
		if u, err := user.Lookup(owner); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		}
	}
	if group != "" {
		if g, err := user.LookupGroup(group); err == nil {
			gid, _ = strconv.Atoi(g.Gid)
		}
	}

	_ = os.Chown(path, uid, gid)
}
