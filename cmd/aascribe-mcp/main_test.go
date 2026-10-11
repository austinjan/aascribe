package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestRejectsPublicPlainHTTPAndPartialTLS(t *testing.T) {
	for _, args := range [][]string{{"--root", ".", "--listen", "0.0.0.0:8080"}, {"--root", ".", "--tls-cert", "cert.pem"}} {
		err := run(context.Background(), args, io.Discard)
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "tls") {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}
