package omprun

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestThinkingHeartbeatThrottlingAndPrivacy(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	delta := []byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_delta","delta":"private reasoning"}}`)
	s.handleRecord(delta)
	s.handleRecord(delta)
	if len(s.events) != 1 {
		t.Fatal("reasoning deltas must be throttled")
	}
	s.lastThinkingActivity = time.Now().Add(-2 * time.Second)
	s.handleRecord(delta)
	if len(s.events) != 2 {
		t.Fatal("ongoing reasoning must refresh activity")
	}
	s.handleRecord([]byte(`{"type":"message_update","assistantMessageEvent":{"type":"thinking_start"}}`))
	if len(s.events) != 3 {
		t.Fatal("a new thinking block must report immediately")
	}
	for len(s.events) > 0 {
		e := <-s.events
		b, err := json.Marshal(e)
		if err != nil || e.Type != EventThinkingDelta || e.Text != "" || strings.Contains(string(b), "private reasoning") {
			t.Fatalf("private thinking leaked: %s", b)
		}
	}
}

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
	s.handleRecord([]byte(`{"type":"extension_ui_request","id":"w","method":"setWidget","widgetKey":"autoresearch"}`))
	for _, kind := range []string{"auto_retry_start", "auto_retry_end", "auto_compaction_start", "auto_compaction_end", "extension_ui_request"} {
		s.handleRecord([]byte(`{"type":"` + kind + `","id":"req","method":"confirm"}`))
	}
	// The setWidget presentation update is not forwarded; the confirm dialog is.
	for _, kind := range []string{"retry", "retry_end", "compaction_start", "compaction_end", "extension_ui"} {
		e := <-s.events
		if e.Type != kind || !json.Valid(e.Data) {
			t.Fatal(kind, e)
		}
	}
	if len(s.events) != 0 {
		t.Fatal("unexpected extra event", <-s.events)
	}
}

func TestSettlesOncePerStretchAndCarriesAbort(t *testing.T) {
	s := &Session{events: make(chan Event, 8)}
	// Background work still pending at the yield: wait for session_settled.
	s.handleRecord([]byte(`{"type":"agent_start"}`))
	s.handleRecord([]byte(`{"type":"prompt_result","id":"r1","agentInvoked":true,"status":"aborted","sessionSettled":false}`))
	if e := <-s.events; e.Type != EventAgentStart {
		t.Fatal(e)
	}
	if len(s.events) != 0 {
		t.Fatal("settled before the session was quiet", <-s.events)
	}
	s.handleRecord([]byte(`{"type":"session_settled"}`))
	if e := <-s.events; e.Type != EventSettled || !e.Aborted {
		t.Fatal("abort status lost", e)
	}
	// Already quiet at the yield: settle immediately and ignore the trailing
	// session_settled frame.
	s.handleRecord([]byte(`{"type":"agent_start"}`))
	<-s.events
	s.handleRecord([]byte(`{"type":"prompt_result","id":"r2","agentInvoked":true,"status":"completed","sessionSettled":true}`))
	s.handleRecord([]byte(`{"type":"session_settled"}`))
	if e := <-s.events; e.Type != EventSettled || e.Aborted {
		t.Fatal(e)
	}
	if len(s.events) != 0 {
		t.Fatal("settled twice", <-s.events)
	}
}

func chunkFrames(t *testing.T, id string, record string, size int) []string {
	t.Helper()
	var parts []string
	for i := 0; i < len(record); i += size {
		parts = append(parts, record[i:min(i+size, len(record))])
	}
	frames := make([]string, len(parts))
	for i, p := range parts {
		b, _ := json.Marshal(chunkFrame{ChunkID: id, Index: i, Count: len(parts), ByteLength: len(record), Data: base64.StdEncoding.EncodeToString([]byte(p))})
		frames[i] = strings.Replace(string(b), "{", `{"type":"rpc_chunk",`, 1)
	}
	return frames
}

func TestChunkedRecordsReassembleAndRejectInterleaving(t *testing.T) {
	s := &Session{events: make(chan Event, 8), pending: map[string]chan *rpcResponse{}}
	reply := make(chan *rpcResponse, 1)
	s.pending["r1"] = reply
	models := `{"type":"response","id":"r1","command":"get_available_models","success":true,"data":{"models":[{"provider":"p","id":"` + strings.Repeat("é", 3000) + `"}]}}`
	for _, f := range chunkFrames(t, "c1", models, 1000) {
		s.handleFrame([]byte(f))
	}
	select {
	case r := <-reply:
		if !r.Success || !strings.Contains(string(r.Data), "éé") {
			t.Fatal("reassembled response corrupted")
		}
	default:
		t.Fatal("chunked response was not delivered")
	}

	// An ordinary frame inside a sequence drops the partial record, reports
	// it, and is itself still processed.
	frames := chunkFrames(t, "c2", `{"type":"agent_end","messages":[]}`, 10)
	s.handleFrame([]byte(frames[0]))
	s.handleFrame([]byte(`{"type":"agent_start"}`))
	for _, f := range frames[1:] {
		s.handleFrame([]byte(f))
	}
	if e := <-s.events; e.Type != "diagnostic" {
		t.Fatal("interleaving not reported", e)
	}
	if e := <-s.events; e.Type != EventAgentStart {
		t.Fatal("interrupting frame lost", e)
	}
	if e := <-s.events; e.Type != "diagnostic" {
		t.Fatal("orphaned continuation chunk not rejected", e)
	}
	for len(s.events) > 0 {
		if e := <-s.events; e.Type == EventAgentEnd {
			t.Fatal("corrupt sequence produced a record")
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

func TestLocalMCPServersFollowOmpTransportInference(t *testing.T) {
	cfg := `{"mcpServers":{
		"kite":{"type":"stdio","command":"npx"},
		"serper":{"command":"npx"},
		"tinyfish":{"type":"http","url":"https://agent.tinyfish.ai/mcp"},
		"bare-url":{"url":"https://example.com/mcp"},
		"legacy":{"type":"sse","url":"https://example.com/sse"},
		"off":{"command":"npx","enabled":false},
		"denied":{"command":"npx"}
	},"disabledServers":["denied"]}`
	got, err := parseLocalMCP([]byte(cfg))
	if err != nil || strings.Join(got, ",") != "kite,serper" {
		t.Fatalf("local servers = %v, %v", got, err)
	}
}
