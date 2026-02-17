package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"fuzoj/internal/cli/command"
	httpclient "fuzoj/internal/cli/http"
	"fuzoj/internal/cli/state"
	pkgerrors "fuzoj/pkg/errors"

	"github.com/google/shlex"
)

// Session holds REPL state.
type Session struct {
	client       *httpclient.Client
	commands     map[string]command.Command
	tokenState   *state.TokenState
	statePath    string
	prettyJSON   bool
	outputWriter *bufio.Writer
}

func New(client *httpclient.Client, commands map[string]command.Command, tokenState *state.TokenState, statePath string, prettyJSON bool) *Session {
	return &Session{
		client:       client,
		commands:     commands,
		tokenState:   tokenState,
		statePath:    statePath,
		prettyJSON:   prettyJSON,
		outputWriter: bufio.NewWriter(os.Stdout),
	}
}

func (s *Session) Run(ctx context.Context) {
	reader := bufio.NewReader(os.Stdin)
	for {
		_, _ = s.outputWriter.WriteString("fuzoj> ")
		_ = s.outputWriter.Flush()
		line, err := reader.ReadString('\n')
		if err != nil {
			s.printLine("read input failed: %v", err)
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if s.handleSystemCommand(line) {
			continue
		}

		if err := s.handleCommand(ctx, reader, line); err != nil {
			s.printLine("error: %v", err)
		}
	}
}

func (s *Session) handleSystemCommand(line string) bool {
	switch line {
	case "exit", "quit":
		s.printLine("bye")
		os.Exit(0)
	case "help":
		s.printHelp()
		return true
	}
	if strings.HasPrefix(line, "set ") {
		s.handleSet(strings.TrimSpace(strings.TrimPrefix(line, "set ")))
		return true
	}
	if strings.HasPrefix(line, "show ") {
		s.handleShow(strings.TrimSpace(strings.TrimPrefix(line, "show ")))
		return true
	}
	return false
}

func (s *Session) handleSet(args string) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		s.printLine("usage: set base|token|timeout")
		return
	}
	switch parts[0] {
	case "base":
		if len(parts) < 2 {
			s.printLine("usage: set base http://127.0.0.1:8080")
			return
		}
		s.client.SetBaseURL(parts[1])
		s.printLine("base set to %s", parts[1])
	case "timeout":
		if len(parts) < 2 {
			s.printLine("usage: set timeout 10s")
			return
		}
		dur, err := time.ParseDuration(parts[1])
		if err != nil {
			s.printLine("invalid duration: %v", err)
			return
		}
		s.client.SetTimeout(dur)
		s.printLine("timeout set to %s", dur)
	case "token":
		if len(parts) < 2 {
			s.printLine("usage: set token <access_token>")
			return
		}
		s.tokenState.AccessToken = parts[1]
		if err := state.Save(s.statePath, *s.tokenState); err != nil {
			s.printLine("save token failed: %v", err)
			return
		}
		s.printLine("token updated")
	default:
		s.printLine("unknown set command")
	}
}

func (s *Session) handleShow(args string) {
	switch args {
	case "token":
		if s.tokenState.AccessToken == "" {
			s.printLine("token: <empty>")
			return
		}
		token := s.tokenState.AccessToken
		if len(token) > 12 {
			token = token[:6] + "..." + token[len(token)-4:]
		}
		s.printLine("token: %s", token)
	case "config":
		s.printLine("tokenStatePath: %s", s.statePath)
	default:
		s.printLine("usage: show token|config")
	}
}

func (s *Session) handleCommand(ctx context.Context, reader *bufio.Reader, line string) error {
	tokens, err := shlex.Split(line)
	if err != nil {
		return fmt.Errorf("parse command failed: %w", err)
	}
	if len(tokens) < 2 {
		return fmt.Errorf("invalid command, use: <service> <action> key=value ...")
	}
	service := tokens[0]
	action := tokens[1]
	key := fmt.Sprintf("%s %s", service, action)
	cmd, ok := s.commands[key]
	if !ok {
		return fmt.Errorf("unknown command: %s %s", service, action)
	}
	params := command.Params{}
	for _, token := range tokens[2:] {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid param: %s", token)
		}
		params.Set(parts[0], parts[1])
	}

	s.applyParamShortcuts(&cmd, params)
	if err := s.promptMissing(reader, &cmd, params); err != nil {
		return err
	}
	req, err := command.BuildRequest(cmd, params)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, req.Method, req.Path, req.Headers, req.Body)
	if err != nil {
		return err
	}
	s.renderResponse(resp)
	s.updateTokenFromResponse(cmd, resp.Body)
	return nil
}

func (s *Session) applyParamShortcuts(cmd *command.Command, params command.Params) {
	if cmd.Service == "submit" && cmd.Action == "create" {
		if params.Get("source_file") != "" && params.Get("source_code") == "" {
			params.Set("source_code", "_file_")
		}
	}
	if cmd.Service == "problem" && cmd.Action == "upload-complete" {
		if params.Get("parts_file") != "" && params.Get("parts_json") == "" {
			params.Set("parts_json", "_file_")
		}
		if params.Get("manifest_file") != "" && params.Get("manifest_json") == "" {
			params.Set("manifest_json", "_file_")
		}
		if params.Get("config_file") != "" && params.Get("config_json") == "" {
			params.Set("config_json", "_file_")
		}
	}
}

func (s *Session) promptMissing(reader *bufio.Reader, cmd *command.Command, params command.Params) error {
	for _, field := range cmd.Fields {
		if !field.Required {
			continue
		}
		if params.Has(field.Name) && params.Get(field.Name) != "" && params.Get(field.Name) != "_file_" {
			continue
		}
		if params.Get(field.Name) == "_file_" {
			continue
		}
		value, err := s.promptValue(reader, field.Prompt)
		if err != nil {
			return err
		}
		params.Set(field.Name, value)
	}
	return nil
}

func (s *Session) promptValue(reader *bufio.Reader, prompt string) (string, error) {
	s.printLine("%s:", prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input failed: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func (s *Session) renderResponse(resp httpclient.ResponseInfo) {
	s.printLine("HTTP %d (%s)", resp.StatusCode, resp.Duration)
	if len(resp.Body) == 0 {
		return
	}
	if s.prettyJSON {
		var raw interface{}
		if err := json.Unmarshal(resp.Body, &raw); err == nil {
			formatted, _ := json.MarshalIndent(raw, "", "  ")
			s.printLine("%s", string(formatted))
			return
		}
	}
	s.printLine("%s", string(resp.Body))
}

func (s *Session) updateTokenFromResponse(cmd command.Command, body []byte) {
	if cmd.Service != "user" {
		return
	}
	type authData struct {
		AccessToken      string    `json:"access_token"`
		RefreshToken     string    `json:"refresh_token"`
		AccessExpiresAt  time.Time `json:"access_expires_at"`
		RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	}
	type respEnvelope struct {
		Code int      `json:"code"`
		Data authData `json:"data"`
	}
	var resp respEnvelope
	if err := json.Unmarshal(body, &resp); err != nil {
		return
	}
	if resp.Code != int(pkgerrors.Success) {
		return
	}
	switch cmd.Action {
	case "login", "register", "refresh":
		if resp.Data.AccessToken != "" {
			s.tokenState.AccessToken = resp.Data.AccessToken
		}
		if resp.Data.RefreshToken != "" {
			s.tokenState.RefreshToken = resp.Data.RefreshToken
		}
		s.tokenState.AccessExpiresAt = resp.Data.AccessExpiresAt
		s.tokenState.RefreshExpiresAt = resp.Data.RefreshExpiresAt
		_ = state.Save(s.statePath, *s.tokenState)
	case "logout":
		s.tokenState.AccessToken = ""
		s.tokenState.RefreshToken = ""
		s.tokenState.AccessExpiresAt = time.Time{}
		s.tokenState.RefreshExpiresAt = time.Time{}
		_ = state.Clear(s.statePath)
	}
}

func (s *Session) printHelp() {
	s.printLine("usage: <service> <action> key=value ...")
	s.printLine("system: help | exit | set base|timeout|token | show token|config")
	s.printLine("examples:")
	s.printLine("  user login username=demo password=secret")
	s.printLine("  problem create title=\"Two Sum\" owner_id=1")
	s.printLine("  submit create problem_id=1 user_id=2 language_id=cpp source_file=./main.cpp")
}

func (s *Session) printLine(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(s.outputWriter, format+"\n", args...)
	_ = s.outputWriter.Flush()
}
