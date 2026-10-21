package pirun

import (
	"bufio"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestJSONLFramingKeepsUnicodeAndLongRecords(t *testing.T) {
	source := "{\"text\":\"" + strings.Repeat("x", 100000) + "\u2028\u2029end\"}\r\n{\"next\":true}\n"
	reader := bufio.NewReaderSize(strings.NewReader(source), 1024)
	first, err := readLine(reader)
	if err != nil || !json.Valid(first) || !strings.Contains(string(first), "\u2028\u2029") {
		t.Fatalf("framing corrupted record: %v", err)
	}
	second, err := readLine(reader)
	if err != nil || string(second) != "{\"next\":true}" {
		t.Fatalf("lost second record: %q %v", second, err)
	}
	if _, err = readLine(reader); err != io.EOF {
		t.Fatal(err)
	}
}

func TestToolLifecycleKeepsIdentitySnapshotsAndDuration(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	records := []string{
		`{"type":"tool_execution_start","toolCallId":"a","toolName":"bash","args":{"command":"echo test"}}`,
		`{"type":"tool_execution_start","toolCallId":"b","toolName":"read","args":{"path":"index.html"}}`,
		`{"type":"tool_execution_update","toolCallId":"a","toolName":"bash","partialResult":{"content":[{"type":"text","text":"first"}]}}`,
		`{"type":"tool_execution_update","toolCallId":"a","toolName":"bash","partialResult":{"content":[{"type":"text","text":"first and second"}]}}`,
		`{"type":"tool_execution_end","toolCallId":"b","toolName":"read","result":{"content":[{"type":"text","text":"file"}]} ,"durationMs":25}`,
		`{"type":"tool_execution_end","toolCallId":"a","toolName":"bash","isError":true,"durationMs":100,"result":{"content":[{"type":"image","data":"secret-binary"},{"type":"text","text":"failed"}],"details":{"exitCode":1}}}`,
	}
	for _, record := range records {
		s.handleRecord([]byte(record))
	}
	for i := 0; i < 6; i++ {
		e := <-s.events
		switch i {
		case 0:
			if e.ToolCallID != "a" || e.Type != EventToolStart {
				t.Fatalf("start: %+v", e)
			}
		case 2:
			if e.Output != "first" || e.ToolCallID != "a" {
				t.Fatal(e)
			}
		case 3:
			if e.Output != "first and second" {
				t.Fatal("partial snapshot was altered")
			}
		case 4:
			if e.ToolCallID != "b" || e.DurationMS != 25 {
				t.Fatal(e)
			}
		case 5:
			if e.ToolCallID != "a" || !e.IsError || e.Output != "failed" || e.DurationMS != 100 || !json.Valid(e.ToolDetails) {
				t.Fatal(e)
			}
		}
	}
}
func TestAuthoritativeTextAndThinkingPrivacy(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	s.handleRecord([]byte(`{"type":"message_start","message":{"role":"assistant"}}`))
	s.handleRecord([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start","contentIndex":0}}`))
	s.handleRecord([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"private reasoning"}}`))
	s.handleRecord([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"draft"}}`))
	s.handleRecord([]byte(`{"type":"message_end","message":{"role":"assistant","stopReason":"stop","content":[{"type":"thinking","thinking":"private reasoning"},{"type":"text","text":"authoritative"}]}}`))
	if e := <-s.events; e.Type != "message_start" {
		t.Fatal(e)
	}
	if e := <-s.events; e.Type != EventThinkingDelta || e.Text != "" {
		t.Fatal("thinking content escaped", e)
	}
	if e := <-s.events; e.Text != "draft" {
		t.Fatal(e)
	}
	if e := <-s.events; e.Type != "message_end" || e.Text != "authoritative" {
		t.Fatal(e)
	}
	if e := <-s.events; e.Type != EventModelResult {
		t.Fatal(e)
	}
}
func TestRetryCompactionAndExtensionRequests(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	for _, kind := range []string{"auto_retry_start", "auto_retry_end", "compaction_start", "compaction_end", "extension_ui_request"} {
		s.handleRecord([]byte(`{"type":"` + kind + `","id":"req","method":"confirm"}`))
	}
	for _, kind := range []string{"retry", "retry_end", "compaction_start", "compaction_end", "extension_ui"} {
		e := <-s.events
		if e.Type != kind || !json.Valid(e.Data) {
			t.Fatal(e)
		}
	}
}
func TestEventBackpressureDoesNotDropAndCloseUnblocks(t *testing.T) {
	s := &Session{events: make(chan Event, 1), stopDelivery: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 2000; i++ {
			s.emit(Event{Type: EventToolStart, ToolCallID: strings.Repeat("x", i%3)})
		}
		close(done)
	}()
	for i := 0; i < 2000; i++ {
		select {
		case <-s.events:
		case <-time.After(time.Second):
			t.Fatal("lost event", i)
		}
	}
	<-done
	s.events <- Event{}
	blocked := make(chan struct{})
	go func() { s.emit(Event{Type: EventSettled}); close(blocked) }()
	// A closed delivery signal releases a reader even when the UI stopped reading.
	close(s.stopDelivery)
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("blocked reader did not stop")
	}
}
