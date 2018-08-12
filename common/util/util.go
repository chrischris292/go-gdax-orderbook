package util

import (
	"bytes"
	"compress/gzip"
	"os"
	"strconv"
	"strings"

	raven "github.com/getsentry/raven-go"
	"go.uber.org/zap"
)

func NumDecPlaces(v float64) int {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	i := strings.IndexByte(s, '.')
	if i > -1 {
		return len(s) - i - 1
	}
	return 0
}

func InitializeZap(config zap.Config) {
	logger, err := config.Build()
	if err != nil {
		raven.CaptureError(err, nil)
		panic(err)
	}

	_ = zap.ReplaceGlobals(logger)

}

func WriteCompressedLine(msg string, fPtr *os.File) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write([]byte(msg + "\n"))
	w.Close() // You must close this first to flush the bytes to the buffer.
	fPtr.Write(b.Bytes())

}
