package myvocalwss

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClientLifecycleDecodesAudioAndFinishesNormally(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("API-Key") != "test-key" {
				t.Errorf("missing API-Key header")
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()
			send := func(value interface{}) {
				if err := conn.WriteJSON(value); err != nil {
					t.Error(err)
				}
			}
			send(map[string]interface{}{"event": "connected_success"})
			for {
				var message map[string]interface{}
				if err := conn.ReadJSON(&message); err != nil {
					return
				}
				switch message["event"] {
				case "task_start":
					send(map[string]interface{}{
						"event":      "task_started",
						"session_id": "session-001",
					})
				case "task_cancel":
					send(map[string]interface{}{
						"event": "task_canceled",
					})
				case "task_continue":
					for index := 0; index < 20; index++ {
						send(map[string]interface{}{
							"event": "task_continued",
							"data": map[string]interface{}{
								"audio": "010203",
							},
							"is_final": index == 19,
						})
					}
				case "task_flush":
					send(map[string]interface{}{"event": "task_flushed"})
				case "task_finish":
					send(map[string]interface{}{"event": "task_finished"})
					_ = conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(1000, "complete"),
						time.Now().Add(time.Second),
					)
					return
				}
			}
		}))
	defer server.Close()

	endpoint := "ws" + strings.TrimPrefix(server.URL, "http")
	var audio []byte
	client := NewClient(endpoint, "test-key", Handlers{
		OnAudio: func(chunk []byte, _ Event) {
			audio = append(audio, chunk...)
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx, StartRequest{
		Voice:     "voice-a",
		SessionID: "session-001",
	}); err != nil {
		t.Fatal(err)
	}
	if err := client.Cancel(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.SendText("Hello."); err != nil {
		t.Fatal(err)
	}
	if err := client.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	result, err := client.Finish(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(audio) != 60 {
		t.Fatalf("audio bytes = %d", len(audio))
	}
	if result.CloseCode != 1000 {
		t.Fatalf("close code = %d", result.CloseCode)
	}
	if result.SessionID != "session-001" {
		t.Fatalf("session id = %s", result.SessionID)
	}
	if client.State() != StateClosed {
		t.Fatalf("state = %s", client.State())
	}
}

func TestSendTextRejectsMoreThan10000Characters(t *testing.T) {
	client := &Client{state: StateStarted}
	if err := client.SendText(strings.Repeat("a", 10001)); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestEventRejectsInvalidHexAudio(t *testing.T) {
	var event Event
	if err := json.Unmarshal([]byte(
		`{"event":"task_continued","data":{"audio":"not-hex"}}`,
	), &event); err != nil {
		t.Fatal(err)
	}
	if _, err := event.AudioBytes(); err == nil {
		t.Fatal("expected hex decode error")
	}
}

func TestDuplicateControlEventsDoNotBlockReadLoop(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				var message map[string]interface{}
				if err := conn.ReadJSON(&message); err != nil {
					return
				}
				switch message["event"] {
				case "task_start":
					for index := 0; index < 40; index++ {
						_ = conn.WriteJSON(map[string]string{
							"event": "task_started",
						})
					}
				case "task_finish":
					_ = conn.WriteJSON(map[string]string{
						"event": "task_finished",
					})
					_ = conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(1000, "complete"),
						time.Now().Add(time.Second),
					)
					return
				}
			}
		}))
	defer server.Close()

	client := NewClient(
		"ws"+strings.TrimPrefix(server.URL, "http"),
		"test-key",
		Handlers{},
	)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx, StartRequest{Voice: "voice-a"}); err != nil {
		t.Fatal(err)
	}
	result, err := client.Finish(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.CloseCode != 1000 {
		t.Fatalf("close code = %d", result.CloseCode)
	}
}

func TestCallbackPanicIsReportedWithoutPoisoningCompletion(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			for {
				var message map[string]interface{}
				if err := conn.ReadJSON(&message); err != nil {
					return
				}
				switch message["event"] {
				case "task_start":
					_ = conn.WriteJSON(map[string]string{
						"event": "task_started",
					})
				case "task_finish":
					_ = conn.WriteJSON(map[string]string{
						"event": "task_finished",
					})
					_ = conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(1000, "complete"),
						time.Now().Add(time.Second),
					)
					return
				}
			}
		}))
	defer server.Close()

	var callbackErrors int32
	client := NewClient(
		"ws"+strings.TrimPrefix(server.URL, "http"),
		"test-key",
		Handlers{
			OnEvent: func(event Event) {
				if event.Event == "task_finished" {
					panic("callback boom")
				}
			},
			OnError: func(error) {
				atomic.AddInt32(&callbackErrors, 1)
			},
		},
	)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx, StartRequest{Voice: "voice-a"}); err != nil {
		t.Fatal(err)
	}
	result, err := client.Finish(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result.CloseCode != 1000 {
		t.Fatalf("close code = %d", result.CloseCode)
	}
	if atomic.LoadInt32(&callbackErrors) != 1 {
		t.Fatalf("callback errors = %d", callbackErrors)
	}
}

func TestTaskFailedAudioIsNotAddedToResultBuffer(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			_, _, _ = conn.ReadMessage()
			_ = conn.WriteJSON(map[string]interface{}{
				"event":      "task_failed",
				"error_code": "2204",
				"error":      "rejected continue",
				"data": map[string]string{
					"audio": "010203",
				},
			})
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(1000, "failed"),
				time.Now().Add(time.Second),
			)
		}))
	defer server.Close()

	client := NewClient(
		"ws"+strings.TrimPrefix(server.URL, "http"),
		"test-key",
		Handlers{},
	)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	err := client.Start(ctx, StartRequest{Voice: "voice-a"})
	if err == nil || !strings.Contains(err.Error(), "2204") {
		t.Fatalf("expected 2204, got %v", err)
	}
	client.mu.RLock()
	audioBytes := client.audio.Len()
	client.mu.RUnlock()
	if audioBytes != 0 {
		t.Fatalf("task_failed audio bytes = %d", audioBytes)
	}
}
