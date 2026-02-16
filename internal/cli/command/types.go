package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// FieldType describes input type.
type FieldType int

const (
	FieldString FieldType = iota
	FieldInt
	FieldInt64
	FieldStringList
	FieldIntList
	FieldJSON
	FieldFile
)

// Field defines a CLI input field.
type Field struct {
	Name     string
	Aliases  []string
	Prompt   string
	Type     FieldType
	Required bool
}

// Command defines a CLI command binding.
type Command struct {
	Service      string
	Action       string
	Method       string
	PathTemplate string
	RequiresAuth bool
	Fields       []Field
}

// RequestSpec is the built HTTP request.
type RequestSpec struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// Params holds parsed input params.
type Params map[string]string

func (p Params) Get(key string) string {
	return p[strings.ToLower(key)]
}

func (p Params) Set(key, value string) {
	p[strings.ToLower(key)] = value
}

func (p Params) Has(key string) bool {
	_, ok := p[strings.ToLower(key)]
	return ok
}

func (p Params) Canonicalize(fields []Field) {
	for _, field := range fields {
		for _, alias := range field.Aliases {
			aliasKey := strings.ToLower(alias)
			if value, ok := p[aliasKey]; ok {
				p[strings.ToLower(field.Name)] = value
				delete(p, aliasKey)
			}
		}
	}
}

func ParseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

func ParseInt(value string) (int, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32)
	return int(n), err
}

func ParseStringList(value string) []string {
	raw := strings.Split(value, ",")
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

func ParseIntList(value string) ([]int, error) {
	items := ParseStringList(value)
	result := make([]int, 0, len(items))
	for _, item := range items {
		n, err := ParseInt(item)
		if err != nil {
			return nil, fmt.Errorf("invalid int list value: %w", err)
		}
		result = append(result, n)
	}
	return result, nil
}

func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	return string(data), nil
}

func ParseJSON(value string) (json.RawMessage, error) {
	raw := strings.TrimSpace(value)
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("invalid json content")
	}
	return json.RawMessage(raw), nil
}
