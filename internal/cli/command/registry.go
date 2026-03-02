package command

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Registry returns all CLI commands keyed by "service action".
func Registry() map[string]Command {
	commands := []Command{
		{
			Service:      "user",
			Action:       "register",
			Method:       "POST",
			PathTemplate: "/api/v1/user/register",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "username", Prompt: "username", Type: FieldString, Required: true},
				{Name: "password", Prompt: "password", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "user",
			Action:       "login",
			Method:       "POST",
			PathTemplate: "/api/v1/user/login",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "username", Prompt: "username", Type: FieldString, Required: true},
				{Name: "password", Prompt: "password", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "user",
			Action:       "refresh",
			Method:       "POST",
			PathTemplate: "/api/v1/user/refresh-token",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "refresh_token", Prompt: "refresh_token", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "user",
			Action:       "logout",
			Method:       "POST",
			PathTemplate: "/api/v1/user/logout",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "refresh_token", Prompt: "refresh_token", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "create",
			Method:       "POST",
			PathTemplate: "/api/v1/problems",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "title", Prompt: "title", Type: FieldString, Required: true},
				{Name: "owner_id", Prompt: "owner_id", Type: FieldInt64, Required: false},
			},
		},
		{
			Service:      "problem",
			Action:       "latest",
			Method:       "GET",
			PathTemplate: "/api/v1/problems/:id/latest",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "statement",
			Method:       "GET",
			PathTemplate: "/api/v1/problems/:id/statement",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "statement-version",
			Method:       "GET",
			PathTemplate: "/api/v1/problems/:id/versions/:version/statement",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "version", Prompt: "version", Type: FieldInt, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "statement-update",
			Method:       "PUT",
			PathTemplate: "/api/v1/problems/:id/versions/:version/statement",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "version", Prompt: "version", Type: FieldInt, Required: true},
				{Name: "statement_md", Prompt: "statement_md", Type: FieldString, Required: false},
				{Name: "statement_file", Prompt: "statement_file", Type: FieldFile, Required: false},
			},
		},
		{
			Service:      "problem",
			Action:       "delete",
			Method:       "DELETE",
			PathTemplate: "/api/v1/problems/:id",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "upload-prepare",
			Method:       "POST",
			PathTemplate: "/api/v1/problems/:id/data-pack/uploads:prepare",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "idempotency_key", Prompt: "idempotency_key", Type: FieldString, Required: true},
				{Name: "expected_size_bytes", Prompt: "expected_size_bytes", Type: FieldInt64, Required: true},
				{Name: "expected_sha256", Prompt: "expected_sha256", Type: FieldString, Required: true},
				{Name: "content_type", Prompt: "content_type", Type: FieldString, Required: true},
				{Name: "created_by", Prompt: "created_by", Type: FieldInt64, Required: true},
				{Name: "client_type", Prompt: "client_type", Type: FieldString, Required: false},
				{Name: "upload_strategy", Prompt: "upload_strategy", Type: FieldString, Required: false},
			},
		},
		{
			Service:      "problem",
			Action:       "upload-sign",
			Method:       "POST",
			PathTemplate: "/api/v1/problems/:id/data-pack/uploads/:upload_id/sign",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "upload_id", Prompt: "upload_id", Type: FieldInt64, Required: true},
				{Name: "part_numbers", Prompt: "part_numbers (comma-separated)", Type: FieldIntList, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "upload-complete",
			Method:       "POST",
			PathTemplate: "/api/v1/problems/:id/data-pack/uploads/:upload_id/complete",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "upload_id", Prompt: "upload_id", Type: FieldInt64, Required: true},
				{Name: "parts_json", Prompt: "parts_json (JSON array)", Type: FieldJSON, Required: true},
				{Name: "manifest_json", Prompt: "manifest_json (JSON)", Type: FieldJSON, Required: true},
				{Name: "config_json", Prompt: "config_json (JSON)", Type: FieldJSON, Required: true},
				{Name: "manifest_hash", Prompt: "manifest_hash", Type: FieldString, Required: true},
				{Name: "data_pack_hash", Prompt: "data_pack_hash", Type: FieldString, Required: true},
				{Name: "parts_file", Prompt: "parts_file", Type: FieldFile, Required: false},
				{Name: "manifest_file", Prompt: "manifest_file", Type: FieldFile, Required: false},
				{Name: "config_file", Prompt: "config_file", Type: FieldFile, Required: false},
			},
		},
		{
			Service:      "problem",
			Action:       "upload-abort",
			Method:       "POST",
			PathTemplate: "/api/v1/problems/:id/data-pack/uploads/:upload_id/abort",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "upload_id", Prompt: "upload_id", Type: FieldInt64, Required: true},
			},
		},
		{
			Service:      "problem",
			Action:       "publish",
			Method:       "POST",
			PathTemplate: "/api/v1/problems/:id/versions/:version/publish",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "version", Prompt: "version", Type: FieldInt, Required: true},
			},
		},
		{
			Service:      "submit",
			Action:       "create",
			Method:       "POST",
			PathTemplate: "/api/v1/submissions",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "problem_id", Prompt: "problem_id", Type: FieldInt64, Required: true},
				{Name: "user_id", Prompt: "user_id", Type: FieldInt64, Required: true},
				{Name: "language_id", Prompt: "language_id", Type: FieldString, Required: true},
				{Name: "source_code", Prompt: "source_code", Type: FieldString, Required: true},
				{Name: "contest_id", Prompt: "contest_id", Type: FieldString, Required: false},
				{Name: "scene", Prompt: "scene", Type: FieldString, Required: false},
				{Name: "extra_compile_flags", Prompt: "extra_compile_flags (comma-separated)", Type: FieldStringList, Required: false},
				{Name: "idempotency_key", Prompt: "idempotency_key", Type: FieldString, Required: false},
				{Name: "source_file", Prompt: "source_file", Type: FieldFile, Required: false},
			},
		},
		{
			Service:      "submit",
			Action:       "status",
			Method:       "GET",
			PathTemplate: "/api/v1/submissions/:id",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "submission_id", Type: FieldString, Required: true},
				{Name: "include", Prompt: "include (details/log)", Type: FieldString, Required: false},
			},
		},
		{
			Service:      "submit",
			Action:       "batch-status",
			Method:       "POST",
			PathTemplate: "/api/v1/submissions/batch_status",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "submission_ids", Prompt: "submission_ids (comma-separated)", Type: FieldStringList, Required: true},
			},
		},
		{
			Service:      "submit",
			Action:       "source",
			Method:       "GET",
			PathTemplate: "/api/v1/submissions/:id/source",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "submission_id", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "judge",
			Action:       "status",
			Method:       "GET",
			PathTemplate: "/api/v1/judge/submissions/:id",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "submission_id", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "contest",
			Action:       "create",
			Method:       "POST",
			PathTemplate: "/api/v1/contests",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "title", Prompt: "title", Type: FieldString, Required: true},
				{Name: "description", Prompt: "description", Type: FieldString, Required: false},
				{Name: "visibility", Prompt: "visibility", Type: FieldString, Required: false},
				{Name: "owner_id", Prompt: "owner_id", Type: FieldInt64, Required: false},
				{Name: "org_id", Prompt: "org_id", Type: FieldInt64, Required: false},
				{Name: "start_at", Prompt: "start_at (RFC3339)", Type: FieldString, Required: false},
				{Name: "end_at", Prompt: "end_at (RFC3339)", Type: FieldString, Required: false},
				{Name: "rule_json", Prompt: "rule_json (JSON)", Type: FieldJSON, Required: false},
			},
		},
		{
			Service:      "contest",
			Action:       "get",
			Method:       "GET",
			PathTemplate: "/api/v1/contests/:id",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "contest",
			Action:       "list",
			Method:       "GET",
			PathTemplate: "/api/v1/contests",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "page", Prompt: "page", Type: FieldInt, Required: false},
				{Name: "page_size", Prompt: "page_size", Type: FieldInt, Required: false},
				{Name: "status", Prompt: "status", Type: FieldString, Required: false},
				{Name: "owner_id", Prompt: "owner_id", Type: FieldInt64, Required: false},
				{Name: "org_id", Prompt: "org_id", Type: FieldInt64, Required: false},
			},
		},
		{
			Service:      "contest",
			Action:       "publish",
			Method:       "POST",
			PathTemplate: "/api/v1/contests/:id/publish",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "contest",
			Action:       "close",
			Method:       "POST",
			PathTemplate: "/api/v1/contests/:id/close",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
			},
		},
		{
			Service:      "contest",
			Action:       "register",
			Method:       "POST",
			PathTemplate: "/api/v1/contests/:id/register",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
				{Name: "user_id", Prompt: "user_id", Type: FieldInt64, Required: true},
				{Name: "team_id", Prompt: "team_id", Type: FieldString, Required: false},
				{Name: "invite_code", Prompt: "invite_code", Type: FieldString, Required: false},
			},
		},
		{
			Service:      "contest",
			Action:       "participants",
			Method:       "GET",
			PathTemplate: "/api/v1/contests/:id/participants",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
				{Name: "page", Prompt: "page", Type: FieldInt, Required: false},
				{Name: "page_size", Prompt: "page_size", Type: FieldInt, Required: false},
			},
		},
		{
			Service:      "contest",
			Action:       "leaderboard",
			Method:       "GET",
			PathTemplate: "/api/v1/contests/:id/leaderboard",
			RequiresAuth: false,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
				{Name: "page", Prompt: "page", Type: FieldInt, Required: false},
				{Name: "page_size", Prompt: "page_size", Type: FieldInt, Required: false},
			},
		},
		{
			Service:      "contest",
			Action:       "my-result",
			Method:       "GET",
			PathTemplate: "/api/v1/contests/:id/my_result",
			RequiresAuth: true,
			Fields: []Field{
				{Name: "id", Prompt: "contest_id", Type: FieldString, Required: true},
			},
		},
	}

	result := make(map[string]Command, len(commands))
	for _, cmd := range commands {
		key := fmt.Sprintf("%s %s", cmd.Service, cmd.Action)
		result[key] = cmd
	}
	return result
}

// BuildRequest creates HTTP request spec based on command.
func BuildRequest(cmd Command, params Params) (RequestSpec, error) {
	params.Canonicalize(cmd.Fields)
	path, err := buildPath(cmd.PathTemplate, params)
	if err != nil {
		return RequestSpec{}, err
	}
	path = appendQuery(path, "include", params.Get("include"))

	headers := map[string]string{}
	if cmd.Service == "problem" && cmd.Action == "upload-prepare" {
		headers["Idempotency-Key"] = params.Get("idempotency_key")
	}
	if cmd.Service == "submit" && cmd.Action == "create" {
		headers["Idempotency-Key"] = params.Get("idempotency_key")
	}

	var body []byte
	if cmd.Method != "GET" && cmd.Method != "DELETE" {
		payload, err := buildPayload(cmd, params)
		if err != nil {
			return RequestSpec{}, err
		}
		if payload != nil {
			body, err = json.Marshal(payload)
			if err != nil {
				return RequestSpec{}, fmt.Errorf("marshal request body failed: %w", err)
			}
		}
	}

	return RequestSpec{
		Method:  cmd.Method,
		Path:    path,
		Headers: headers,
		Body:    body,
	}, nil
}

func appendQuery(path, key, value string) string {
	if strings.TrimSpace(value) == "" {
		return path
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%s%s=%s", path, sep, url.QueryEscape(key), url.QueryEscape(value))
}

func buildPath(template string, params Params) (string, error) {
	path := template
	for _, key := range []string{"id", "upload_id", "version"} {
		placeholder := ":" + key
		if strings.Contains(path, placeholder) {
			value := params.Get(key)
			if value == "" {
				return "", fmt.Errorf("missing path parameter: %s", key)
			}
			path = strings.ReplaceAll(path, placeholder, value)
		}
	}
	return path, nil
}

func buildPayload(cmd Command, params Params) (interface{}, error) {
	switch cmd.Service {
	case "user":
		switch cmd.Action {
		case "register", "login":
			return map[string]string{
				"username": params.Get("username"),
				"password": params.Get("password"),
			}, nil
		case "refresh", "logout":
			return map[string]string{
				"refresh_token": params.Get("refresh_token"),
			}, nil
		}
	case "problem":
		switch cmd.Action {
		case "create":
			payload := map[string]interface{}{
				"title": params.Get("title"),
			}
			if params.Get("owner_id") != "" {
				ownerID, err := ParseInt64(params.Get("owner_id"))
				if err != nil {
					return nil, fmt.Errorf("invalid owner_id: %w", err)
				}
				payload["owner_id"] = ownerID
			}
			return payload, nil
		case "upload-prepare":
			ownerID, err := ParseInt64(params.Get("created_by"))
			if err != nil {
				return nil, fmt.Errorf("invalid created_by: %w", err)
			}
			expectedSize, err := ParseInt64(params.Get("expected_size_bytes"))
			if err != nil {
				return nil, fmt.Errorf("invalid expected_size_bytes: %w", err)
			}
			payload := map[string]interface{}{
				"expected_size_bytes": expectedSize,
				"expected_sha256":     params.Get("expected_sha256"),
				"content_type":        params.Get("content_type"),
				"created_by":          ownerID,
			}
			if params.Get("client_type") != "" {
				payload["client_type"] = params.Get("client_type")
			}
			if params.Get("upload_strategy") != "" {
				payload["upload_strategy"] = params.Get("upload_strategy")
			}
			return payload, nil
		case "upload-sign":
			parts, err := ParseIntList(params.Get("part_numbers"))
			if err != nil {
				return nil, err
			}
			return map[string]interface{}{
				"part_numbers": parts,
			}, nil
		case "upload-complete":
			return buildUploadCompletePayload(params)
		case "statement-update":
			statement := params.Get("statement_md")
			if (statement == "" || statement == "_file_") && params.Get("statement_file") != "" {
				content, err := ReadFile(params.Get("statement_file"))
				if err != nil {
					return nil, err
				}
				statement = content
			}
			if statement == "" {
				return nil, fmt.Errorf("statement_md is required")
			}
			return map[string]interface{}{
				"statement_md": statement,
			}, nil
		}
	case "submit":
		switch cmd.Action {
		case "create":
			return buildSubmitCreatePayload(params)
		case "batch-status":
			ids := ParseStringList(params.Get("submission_ids"))
			return map[string]interface{}{
				"submission_ids": ids,
			}, nil
		}
	case "contest":
		switch cmd.Action {
		case "create":
			payload := map[string]interface{}{
				"title": params.Get("title"),
			}
			if params.Get("description") != "" {
				payload["description"] = params.Get("description")
			}
			if params.Get("visibility") != "" {
				payload["visibility"] = params.Get("visibility")
			}
			if params.Get("owner_id") != "" {
				ownerID, err := ParseInt64(params.Get("owner_id"))
				if err != nil {
					return nil, fmt.Errorf("invalid owner_id: %w", err)
				}
				payload["owner_id"] = ownerID
			}
			if params.Get("org_id") != "" {
				orgID, err := ParseInt64(params.Get("org_id"))
				if err != nil {
					return nil, fmt.Errorf("invalid org_id: %w", err)
				}
				payload["org_id"] = orgID
			}
			if params.Get("start_at") != "" {
				payload["start_at"] = params.Get("start_at")
			}
			if params.Get("end_at") != "" {
				payload["end_at"] = params.Get("end_at")
			}
			if params.Get("rule_json") != "" {
				rulePayload, err := ParseJSON(params.Get("rule_json"))
				if err != nil {
					return nil, err
				}
				payload["rule"] = rulePayload
			}
			return payload, nil
		case "register":
			userID, err := ParseInt64(params.Get("user_id"))
			if err != nil {
				return nil, fmt.Errorf("invalid user_id: %w", err)
			}
			payload := map[string]interface{}{
				"user_id": userID,
			}
			if params.Get("team_id") != "" {
				payload["team_id"] = params.Get("team_id")
			}
			if params.Get("invite_code") != "" {
				payload["invite_code"] = params.Get("invite_code")
			}
			return payload, nil
		}
	}
	return nil, nil
}

func buildSubmitCreatePayload(params Params) (interface{}, error) {
	problemID, err := ParseInt64(params.Get("problem_id"))
	if err != nil {
		return nil, fmt.Errorf("invalid problem_id: %w", err)
	}
	userID, err := ParseInt64(params.Get("user_id"))
	if err != nil {
		return nil, fmt.Errorf("invalid user_id: %w", err)
	}

	sourceCode := params.Get("source_code")
	if (sourceCode == "" || sourceCode == "_file_") && params.Get("source_file") != "" {
		sourceCode, err = ReadFile(params.Get("source_file"))
		if err != nil {
			return nil, err
		}
	}
	if sourceCode == "" {
		return nil, fmt.Errorf("source_code is required")
	}

	payload := map[string]interface{}{
		"problem_id":  problemID,
		"user_id":     userID,
		"language_id": params.Get("language_id"),
		"source_code": sourceCode,
	}
	if params.Get("contest_id") != "" {
		payload["contest_id"] = params.Get("contest_id")
	}
	if params.Get("scene") != "" {
		payload["scene"] = params.Get("scene")
	}
	if params.Get("extra_compile_flags") != "" {
		payload["extra_compile_flags"] = ParseStringList(params.Get("extra_compile_flags"))
	}
	return payload, nil
}

func buildUploadCompletePayload(params Params) (interface{}, error) {
	var partsRaw string
	if params.Get("parts_file") != "" {
		data, err := ReadFile(params.Get("parts_file"))
		if err != nil {
			return nil, err
		}
		partsRaw = data
	} else if params.Get("parts_json") != "_file_" {
		partsRaw = params.Get("parts_json")
	}
	partsJSON, err := ParseJSON(partsRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid parts_json: %w", err)
	}

	manifestJSON, err := parseJSONOrFile(params, "manifest_json", "manifest_file")
	if err != nil {
		return nil, err
	}
	configJSON, err := parseJSONOrFile(params, "config_json", "config_file")
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"parts":          json.RawMessage(partsJSON),
		"manifest_json":  json.RawMessage(manifestJSON),
		"config_json":    json.RawMessage(configJSON),
		"manifest_hash":  params.Get("manifest_hash"),
		"data_pack_hash": params.Get("data_pack_hash"),
	}
	return payload, nil
}

func parseJSONOrFile(params Params, key, fileKey string) (json.RawMessage, error) {
	value := params.Get(key)
	if (value == "" || value == "_file_") && params.Get(fileKey) != "" {
		data, err := ReadFile(params.Get(fileKey))
		if err != nil {
			return nil, err
		}
		value = data
	}
	if value == "" {
		return nil, fmt.Errorf("%s is required", key)
	}
	return ParseJSON(value)
}
