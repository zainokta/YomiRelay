package translation

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var errAppServerClosed = errors.New("codex app-server closed")

type appServerTransport interface {
	call(context.Context, string, any, any) error
	notify(string, any) error
	waitTurn(context.Context, string, string) (string, error)
	close() error
}

type appServerStarter func(string) (appServerTransport, error)

type rpcMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "codex app-server request failed"
	}
	if e.Message == "" {
		return fmt.Sprintf("codex app-server request failed (%d)", e.Code)
	}
	return e.Message
}

type appServer struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	messages chan rpcMessage
	done     chan struct{}
	stop     chan struct{}
	workDir  string

	writeMu  sync.Mutex
	nextID   int64
	closeOne sync.Once
	errMu    sync.Mutex
	err      error
}

func startAppServer(binary string) (appServerTransport, error) {
	workDir, err := os.MkdirTemp("", "yomirelay-codex-app-")
	if err != nil {
		return nil, fmt.Errorf("create app-server directory: %w", err)
	}
	command := exec.Command(binary, "app-server", "--listen", "stdio://", "--disable", "plugins", "--disable", "apps", "--config", `service_tier="fast"`, "--config", "features.fast_mode=true")
	command.Dir = workDir
	command.Stderr = io.Discard
	stdin, err := command.StdinPipe()
	if err != nil {
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("open app-server stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("open app-server stdout: %w", err)
	}
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		_ = os.RemoveAll(workDir)
		return nil, fmt.Errorf("start app-server: %w", err)
	}
	server := &appServer{
		cmd:      command,
		stdin:    stdin,
		messages: make(chan rpcMessage, 64),
		done:     make(chan struct{}),
		stop:     make(chan struct{}),
		workDir:  workDir,
	}
	go server.readLoop(stdout)
	return server, nil
}

func (s *appServer) readLoop(stdout io.ReadCloser) {
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	var readErr error
	for scanner.Scan() {
		var message rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			readErr = fmt.Errorf("decode app-server message: %w", err)
			break
		}
		select {
		case s.messages <- message:
		case <-s.stop:
			return
		}
	}
	if err := scanner.Err(); err != nil {
		readErr = fmt.Errorf("read app-server output: %w", err)
	}
	if readErr != nil {
		s.setError(readErr)
		if s.cmd.Process != nil {
			_ = s.cmd.Process.Kill()
		}
	}
	err := s.cmd.Wait()
	s.setError(err)
	close(s.messages)
	close(s.done)
}

func (s *appServer) setError(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.errMu.Unlock()
}

func (s *appServer) processError() error {
	s.errMu.Lock()
	err := s.err
	s.errMu.Unlock()
	if err != nil {
		return fmt.Errorf("%w: %v", errAppServerClosed, err)
	}
	return errAppServerClosed
}

func (s *appServer) next(ctx context.Context) (rpcMessage, error) {
	select {
	case <-ctx.Done():
		return rpcMessage{}, ctx.Err()
	case message, ok := <-s.messages:
		if !ok {
			return rpcMessage{}, s.processError()
		}
		return message, nil
	}
}

func (s *appServer) send(method string, params any, request bool) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	requestValue := map[string]any{"method": method}
	var id int64
	if request {
		s.nextID++
		id = s.nextID
		requestValue["id"] = id
	}
	if params != nil {
		requestValue["params"] = params
	}
	data, err := json.Marshal(requestValue)
	if err != nil {
		return 0, err
	}
	data = append(data, '\n')
	if _, err := s.stdin.Write(data); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *appServer) call(ctx context.Context, method string, params any, result any) error {
	id, err := s.send(method, params, true)
	if err != nil {
		return err
	}
	for {
		message, err := s.next(ctx)
		if err != nil {
			return err
		}
		if !messageIDIs(message.ID, id) {
			continue
		}
		if message.Error != nil {
			return message.Error
		}
		if result == nil || len(message.Result) == 0 || string(message.Result) == "null" {
			return nil
		}
		if err := json.Unmarshal(message.Result, result); err != nil {
			return fmt.Errorf("decode %s response: %w", method, err)
		}
		return nil
	}
}

func (s *appServer) notify(method string, params any) error {
	_, err := s.send(method, params, false)
	return err
}

func messageIDIs(raw json.RawMessage, want int64) bool {
	if len(raw) == 0 {
		return false
	}
	var got int64
	return json.Unmarshal(raw, &got) == nil && got == want
}

type appServerItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

type appServerTurn struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Error  json.RawMessage `json:"error"`
	Items  []appServerItem `json:"items"`
}

func (s *appServer) waitTurn(ctx context.Context, threadID, turnID string) (string, error) {
	var finalText string
	var delta strings.Builder
	var deltaItemID string
	for {
		message, err := s.next(ctx)
		if err != nil {
			return "", err
		}
		switch message.Method {
		case "item/completed":
			var params struct {
				ThreadID string        `json:"threadId"`
				TurnID   string        `json:"turnId"`
				Item     appServerItem `json:"item"`
			}
			if json.Unmarshal(message.Params, &params) == nil && params.ThreadID == threadID && params.TurnID == turnID && params.Item.Type == "agentMessage" {
				finalText = params.Item.Text
			}
		case "item/agentMessage/delta":
			var params struct {
				ThreadID string `json:"threadId"`
				TurnID   string `json:"turnId"`
				ItemID   string `json:"itemId"`
				Delta    string `json:"delta"`
			}
			if json.Unmarshal(message.Params, &params) == nil && params.ThreadID == threadID && params.TurnID == turnID {
				if deltaItemID == "" {
					deltaItemID = params.ItemID
				}
				if params.ItemID == deltaItemID {
					delta.WriteString(params.Delta)
				}
			}
		case "turn/completed":
			var params struct {
				ThreadID string        `json:"threadId"`
				Turn     appServerTurn `json:"turn"`
			}
			if json.Unmarshal(message.Params, &params) != nil || params.ThreadID != threadID || params.Turn.ID != turnID {
				continue
			}
			if params.Turn.Status != "completed" {
				return "", fmt.Errorf("turn ended with status %q", params.Turn.Status)
			}
			if finalText == "" {
				for _, item := range params.Turn.Items {
					if item.Type == "agentMessage" {
						finalText = item.Text
					}
				}
			}
			if finalText == "" {
				finalText = delta.String()
			}
			if finalText == "" {
				return "", errors.New("turn completed without an agent message")
			}
			return finalText, nil
		}
	}
}

func (s *appServer) close() error {
	var closeErr error
	s.closeOne.Do(func() {
		close(s.stop)
		if err := s.stdin.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			closeErr = err
		}
		if s.cmd.Process != nil {
			if err := s.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) && closeErr == nil {
				closeErr = err
			}
		}
	})
	select {
	case <-s.done:
		_ = os.RemoveAll(s.workDir)
	case <-time.After(time.Second):
		if closeErr == nil {
			closeErr = errors.New("timed out closing codex app-server")
		}
	}
	return closeErr
}
