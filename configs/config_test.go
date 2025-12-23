// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package config

import (
	"os"
	"testing"
)

/*
No test case for GetToken yet due to token_manager dependency
Mocking token_manager isn't fun

TestLoadConfig focuses on verifying main functionality (e.g. fallback when invalid input),
and *shouldn't* need exhaustive scenarios
*/

// TestLoadConfig is a test function that verifies the behavior of the LoadConfig function.
func TestLoadConfig(t *testing.T) {
	t.Parallel()
	// Helper function to set environment variables
	setEnv := func(env map[string]string) {
		for k, v := range env {
			t.Setenv(k, v)
		}
	}

	// Helper function to unset environment variables
	unsetEnv := func(env map[string]string) {
		for k := range env {
			os.Unsetenv(k)
		}
	}

	tests := []struct {
		name    string            // Description of the test case
		env     map[string]string // Name of the environment variable and its value
		wantErr bool              // Whether an error is expected
	}{
		{
			name: "Valid configuration",
			env: map[string]string{
				"PIXIVFE_HOST":  "localhost",
				"PIXIVFE_PORT":  "8282",
				"PIXIVFE_TOKEN": "token1,token2",
			},
			wantErr: false,
		},
		{
			name: "Missing required PIXIVFE_TOKEN",
			env: map[string]string{
				"PIXIVFE_HOST":       "localhost",
				"PIXIVFE_PORT":       "8282",
				"PIXIVFE_IMAGEPROXY": "https://imageproxy.test",
			},
			wantErr: true,
		},
		{
			name: "Invalid PIXIVFE_IMAGEPROXY",
			env: map[string]string{
				"PIXIVFE_HOST":       "localhost",
				"PIXIVFE_PORT":       "8282",
				"PIXIVFE_TOKEN":      "token1,token2",
				"PIXIVFE_IMAGEPROXY": "invalidimageproxy-test",
			},
			wantErr: true,
		},
		{
			name: "Invalid PIXIVFE_TOKEN_LOAD_BALANCING",
			env: map[string]string{
				"PIXIVFE_HOST":                 "localhost",
				"PIXIVFE_PORT":                 "8282",
				"PIXIVFE_TOKEN":                "token1,token2",
				"PIXIVFE_TOKEN_LOAD_BALANCING": "invalid-load-balancing-method",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Set up environment
			setEnv(tt.env)
			defer unsetEnv(tt.env)

			// Create a new ServerConfig instance
			config := &ServerConfig{}

			// Call LoadConfig
			err := config.LoadConfig()

			// Check for errors
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				// Test whether config fields were set correctly
				if config.Basic.Host != tt.env["PIXIVFE_HOST"] {
					t.Errorf("LoadConfig() Host = %v, want %v", config.Basic.Host, tt.env["PIXIVFE_HOST"])
				}

				if config.Basic.Port != tt.env["PIXIVFE_PORT"] {
					t.Errorf("LoadConfig() Port = %v, want %v", config.Basic.Port, tt.env["PIXIVFE_PORT"])
				}

				if len(config.Basic.Token) != 2 && tt.env["PIXIVFE_TOKEN"] == "token1,token2" {
					t.Errorf("LoadConfig() Token count = %v, want 2", len(config.Basic.Token))
				}

				if config.ContentProxies.Image.String() == "" {
					t.Error("LoadConfig() ProxyServer is empty")
				}

				if config.TokenManager.LoadBalancing == "" {
					t.Error("LoadConfig() TokenLoadBalancing is empty")
				}
			}
		})
	}
}
