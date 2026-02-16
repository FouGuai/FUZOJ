package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"fuzoj/internal/cli/command"
	"fuzoj/internal/cli/config"
	"fuzoj/internal/cli/http"
	"fuzoj/internal/cli/repl"
	"fuzoj/internal/cli/state"
)

const defaultConfigPath = "configs/cli.yaml"

func main() {
	configPath := flag.String("config", defaultConfigPath, "Path to config file")
	baseURL := flag.String("base", "", "Override base URL")
	timeout := flag.Duration("timeout", 0, "Override HTTP timeout (e.g. 10s)")
	token := flag.String("token", "", "Override access token")
	statePath := flag.String("state", "", "Override token state path")
	pretty := flag.Bool("pretty", false, "Pretty print JSON response")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		return
	}
	if *baseURL != "" {
		cfg.BaseURL = *baseURL
	}
	if *timeout > 0 {
		cfg.Timeout = *timeout
	}
	if *statePath != "" {
		cfg.TokenStatePath = *statePath
	}
	if *pretty {
		trueValue := true
		cfg.PrettyJSON = &trueValue
	}

	tokenState, err := state.Load(cfg.TokenStatePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load token state failed: %v\n", err)
		return
	}
	if *token != "" {
		tokenState.AccessToken = *token
	}

	client := httpclient.New(cfg.BaseURL, cfg.Timeout, func() string {
		return tokenState.AccessToken
	})

	commands := command.Registry()
	session := repl.New(client, commands, &tokenState, cfg.TokenStatePath, cfg.PrettyJSON != nil && *cfg.PrettyJSON)
	session.Run(context.Background())
}
