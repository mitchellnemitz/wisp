package lsp

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func frame(body string) string {
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func TestReadMessageRoundTrip(t *testing.T) {
	in := frame(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`) +
		frame(`{"jsonrpc":"2.0","method":"initialized","params":{}}`) +
		frame(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`)
	c := newConn(strings.NewReader(in), io.Discard)

	m1, err := c.readMessage()
	if err != nil || !m1.isRequest() || m1.Method != "initialize" {
		t.Fatalf("msg1: %+v err=%v (want request initialize)", m1, err)
	}
	m2, err := c.readMessage()
	if err != nil || m2.isRequest() || m2.Method != "initialized" {
		t.Fatalf("msg2: %+v err=%v (want notification initialized)", m2, err)
	}
	m3, err := c.readMessage()
	if err != nil || !m3.isRequest() || m3.Method != "shutdown" {
		t.Fatalf("msg3: %+v err=%v (want request shutdown)", m3, err)
	}
	if _, err := c.readMessage(); err != io.EOF {
		t.Fatalf("end of stream: err=%v, want io.EOF", err)
	}
}

func TestReadMessageToleratesJunkHeaders(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	// A line with no colon, then an unknown header with a colon, then the real
	// Content-Length. Both should be skipped without error.
	in := "garbage-line-without-colon\r\n" +
		"X-Unknown: whatever\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
	c := newConn(strings.NewReader(in), io.Discard)
	m, err := c.readMessage()
	if err != nil || m.Method != "ping" {
		t.Fatalf("got %+v err=%v, want request ping after junk headers", m, err)
	}
}

func TestReadMessageParseErrorThenContinuedService(t *testing.T) {
	good := `{"jsonrpc":"2.0","id":2,"method":"ok"}`
	in := frame("this is not json") + frame(good)
	c := newConn(strings.NewReader(in), io.Discard)

	if _, err := c.readMessage(); !errors.Is(err, errParseError) {
		t.Fatalf("first read err=%v, want errParseError", err)
	}
	// The stream stays aligned: the next message reads fine (continued service).
	m, err := c.readMessage()
	if err != nil || m.Method != "ok" {
		t.Fatalf("after parse error got %+v err=%v, want request ok", m, err)
	}
}

func TestReadMessageBadFrame(t *testing.T) {
	in := "Content-Length: notanumber\r\n\r\n{}"
	c := newConn(strings.NewReader(in), io.Discard)
	if _, err := c.readMessage(); !errors.Is(err, errBadFrame) {
		t.Fatalf("err=%v, want errBadFrame", err)
	}
}
