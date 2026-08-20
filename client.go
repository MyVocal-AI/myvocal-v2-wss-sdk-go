package myvocalwss

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const DefaultEndpoint = "wss://openapi.myvocal.ai/v2/ai/speech/websocket"

type State string

const (
	StateNew       State = "new"
	StateConnected State = "connected"
	StateStarted   State = "started"
	StateFinishing State = "finishing"
	StateFinished  State = "finished"
	StateClosed    State = "closed"
)

type Handlers struct {
	OnEvent  func(Event)
	OnAudio  func([]byte, Event)
	OnError  func(error)
	OnClosed func(int, string)
}

type CompletionResult struct {
	Audio     []byte
	CloseCode int
	SessionID string
}

type closeResult struct {
	code   int
	reason string
}

type Client struct {
	endpoint string
	apiKey   string
	handlers Handlers
	dialer   *websocket.Dialer

	mu           sync.RWMutex
	writeMu      sync.Mutex
	state        State
	conn         *websocket.Conn
	queues       map[string]chan Event
	terminalErr  error
	errorSignal  chan struct{}
	closed       chan closeResult
	audio        bytes.Buffer
	sessionID    string
	taskFinished bool
	closeOnce    sync.Once
	errorOnce    sync.Once
	stopPing     chan struct{}
}

func NewClient(endpoint, apiKey string, handlers Handlers) *Client {
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	return &Client{
		endpoint:    endpoint,
		apiKey:      apiKey,
		handlers:    handlers,
		dialer:      websocket.DefaultDialer,
		state:       StateNew,
		queues:      make(map[string]chan Event),
		errorSignal: make(chan struct{}),
		closed:      make(chan closeResult, 1),
		stopPing:    make(chan struct{}),
	}
}

func (c *Client) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) Connect(ctx context.Context) error {
	if c.apiKey == "" {
		return errors.New("api key is required")
	}
	if err := c.requireState(StateNew); err != nil {
		return err
	}
	headers := http.Header{}
	headers.Set("API-Key", c.apiKey)
	conn, _, err := c.dialer.DialContext(ctx, c.endpoint, headers)
	if err != nil {
		return fmt.Errorf("connect WebSocket: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.state = StateConnected
	c.mu.Unlock()
	go c.readLoop()
	go c.pingLoop()
	return nil
}

func (c *Client) Start(ctx context.Context, request StartRequest) error {
	if err := c.requireState(StateConnected); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	c.mu.Lock()
	c.sessionID = request.SessionID
	c.mu.Unlock()
	if err := c.writeJSON(request); err != nil {
		return err
	}
	event, err := c.await(ctx, "task_started")
	if err != nil {
		return err
	}
	if event.SessionID != "" {
		c.mu.Lock()
		c.sessionID = event.SessionID
		c.mu.Unlock()
	}
	c.setState(StateStarted)
	return nil
}

func (c *Client) SendText(text string) error {
	if err := c.requireState(StateStarted); err != nil {
		return err
	}
	if len([]rune(text)) < 1 || len([]rune(text)) > 10000 {
		return errors.New("text must contain 1 to 10000 characters")
	}
	return c.writeJSON(map[string]interface{}{
		"event": "task_continue",
		"text":  text,
	})
}

func (c *Client) Cancel(ctx context.Context) error {
	if err := c.requireState(StateStarted); err != nil {
		return err
	}
	if err := c.writeJSON(map[string]string{"event": "task_cancel"}); err != nil {
		return err
	}
	_, err := c.await(ctx, "task_canceled")
	return err
}

func (c *Client) Flush(ctx context.Context) error {
	if err := c.requireState(StateStarted); err != nil {
		return err
	}
	if err := c.writeJSON(map[string]string{"event": "task_flush"}); err != nil {
		return err
	}
	_, err := c.await(ctx, "task_flushed")
	return err
}

func (c *Client) Finish(ctx context.Context) (CompletionResult, error) {
	if err := c.requireState(StateStarted); err != nil {
		return CompletionResult{}, err
	}
	c.setState(StateFinishing)
	if err := c.writeJSON(map[string]string{"event": "task_finish"}); err != nil {
		return CompletionResult{}, err
	}
	event, err := c.await(ctx, "task_finished")
	if err != nil {
		return CompletionResult{}, err
	}
	c.mu.Lock()
	c.taskFinished = true
	if event.SessionID != "" {
		c.sessionID = event.SessionID
	}
	c.state = StateFinished
	c.mu.Unlock()

	var closed closeResult
	select {
	case closed = <-c.closed:
	case <-ctx.Done():
		return CompletionResult{}, fmt.Errorf(
			"task_finished received but close frame was not received: %w",
			ctx.Err(),
		)
	}
	if closed.code != websocket.CloseNormalClosure {
		return CompletionResult{}, fmt.Errorf(
			"unexpected close code after task_finished: %d", closed.code,
		)
	}
	c.mu.Lock()
	result := CompletionResult{
		Audio:     append([]byte(nil), c.audio.Bytes()...),
		CloseCode: closed.code,
		SessionID: c.sessionID,
	}
	c.state = StateClosed
	c.mu.Unlock()
	return result, nil
}

func (c *Client) Close() error {
	c.mu.RLock()
	conn := c.conn
	state := c.state
	c.mu.RUnlock()
	if state == StateClosed {
		return nil
	}
	c.closeOnce.Do(func() { close(c.stopPing) })
	if conn != nil {
		c.writeMu.Lock()
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(
				websocket.CloseNormalClosure, "client close",
			),
			time.Now().Add(5*time.Second),
		)
		c.writeMu.Unlock()
		_ = conn.Close()
	}
	c.setState(StateClosed)
	return nil
}

func (c *Client) readLoop() {
	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			code := websocket.CloseAbnormalClosure
			reason := err.Error()
			if closeError, ok := err.(*websocket.CloseError); ok {
				code = closeError.Code
				reason = closeError.Text
			}
			c.mu.RLock()
			finished := c.taskFinished
			c.mu.RUnlock()
			if !(finished && code == websocket.CloseNormalClosure) {
				c.fail(fmt.Errorf(
					"WebSocket closed before normal completion: %w", err,
				))
			}
			c.signalClosed(closeResult{code: code, reason: reason})
			c.setState(StateClosed)
			if c.handlers.OnClosed != nil {
				c.invokeHandler("OnClosed", func() {
					c.handlers.OnClosed(code, reason)
				})
			}
			return
		}
		var event Event
		if err := json.Unmarshal(payload, &event); err != nil {
			c.fail(fmt.Errorf("decode server event: %w", err))
			continue
		}
		chunk, err := event.AudioBytes()
		if err != nil {
			c.fail(err)
			continue
		}
		c.mu.Lock()
		if event.SessionID != "" {
			c.sessionID = event.SessionID
		}
		if len(chunk) > 0 && event.Event != "task_failed" {
			_, _ = c.audio.Write(chunk)
		}
		if event.Event == "task_finished" {
			c.taskFinished = true
		}
		c.mu.Unlock()
		if c.handlers.OnEvent != nil {
			c.invokeHandler("OnEvent", func() {
				c.handlers.OnEvent(event)
			})
		}
		if len(chunk) > 0 && c.handlers.OnAudio != nil {
			audio := append([]byte(nil), chunk...)
			c.invokeHandler("OnAudio", func() {
				c.handlers.OnAudio(audio, event)
			})
		}
		if event.Event == "task_failed" {
			c.fail(event.protocolError())
			continue
		}
		name := event.Event
		if name == "task_cancelled" {
			name = "task_canceled"
		}
		switch name {
		case "task_started", "task_canceled", "task_flushed",
			"task_finished":
			select {
			case c.queue(name) <- event:
			default:
			}
		}
	}
}

func (c *Client) pingLoop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.writeMu.Lock()
			err := c.conn.WriteControl(
				websocket.PingMessage, nil, time.Now().Add(5*time.Second),
			)
			c.writeMu.Unlock()
			if err != nil {
				c.mu.RLock()
				finished := c.taskFinished
				c.mu.RUnlock()
				if !finished {
					c.fail(fmt.Errorf("send ping: %w", err))
				}
				return
			}
		case <-c.stopPing:
			return
		}
	}
}

func (c *Client) writeJSON(value interface{}) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()
	if conn == nil {
		return errors.New("WebSocket is not connected")
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
	err := conn.WriteJSON(value)
	_ = conn.SetWriteDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("send WebSocket message: %w", err)
	}
	return nil
}

func (c *Client) await(ctx context.Context, eventName string) (Event, error) {
	c.mu.RLock()
	terminalErr := c.terminalErr
	c.mu.RUnlock()
	if terminalErr != nil {
		return Event{}, terminalErr
	}
	select {
	case event := <-c.queue(eventName):
		return event, nil
	case <-c.errorSignal:
		c.mu.RLock()
		defer c.mu.RUnlock()
		return Event{}, c.terminalErr
	case <-ctx.Done():
		return Event{}, fmt.Errorf(
			"waiting for %s: %w", eventName, ctx.Err(),
		)
	}
}

func (c *Client) queue(name string) chan Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	queue, ok := c.queues[name]
	if !ok {
		queue = make(chan Event, 16)
		c.queues[name] = queue
	}
	return queue
}

func (c *Client) fail(err error) {
	c.errorOnce.Do(func() {
		c.mu.Lock()
		c.terminalErr = err
		c.mu.Unlock()
		close(c.errorSignal)
		c.reportHandlerError(err)
	})
}

func (c *Client) signalClosed(result closeResult) {
	select {
	case c.closed <- result:
	default:
	}
	c.closeOnce.Do(func() { close(c.stopPing) })
}

func (c *Client) requireState(expected State) error {
	current := c.State()
	if current != expected {
		return fmt.Errorf(
			"operation requires %s, current=%s", expected, current,
		)
	}
	return nil
}

func (c *Client) setState(state State) {
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()
}

func (c *Client) invokeHandler(name string, callback func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			c.reportHandlerError(fmt.Errorf(
				"%s callback panic: %v", name, recovered,
			))
		}
	}()
	callback()
}

func (c *Client) reportHandlerError(err error) {
	if c.handlers.OnError == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	c.handlers.OnError(err)
}
