package main

import "testing"

func TestParseCLIArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantHelp  bool
		wantVer   bool
		expectErr bool
	}{
		{
			name:     "defaults",
			args:     []string{},
			wantPath: defaultConfigPath,
		},
		{
			name:     "short config flag",
			args:     []string{"-c", "/tmp/custom.yaml"},
			wantPath: "/tmp/custom.yaml",
		},
		{
			name:     "long config flag",
			args:     []string{"--config", "./custom.yaml"},
			wantPath: "./custom.yaml",
		},
		{
			name:     "short version flag",
			args:     []string{"-v"},
			wantPath: defaultConfigPath,
			wantVer:  true,
		},
		{
			name:     "long version flag",
			args:     []string{"--version"},
			wantPath: defaultConfigPath,
			wantVer:  true,
		},
		{
			name:     "short help flag",
			args:     []string{"-h"},
			wantPath: defaultConfigPath,
			wantHelp: true,
		},
		{
			name:     "long help flag",
			args:     []string{"--help"},
			wantPath: defaultConfigPath,
			wantHelp: true,
		},
		{
			name:      "unexpected positional argument",
			args:      []string{"run"},
			expectErr: true,
		},
		{
			name:      "unknown flag",
			args:      []string{"--does-not-exist"},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCLIArgs(tt.args)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseCLIArgs returned error: %v", err)
			}
			if got.configPath != tt.wantPath {
				t.Fatalf("configPath = %q, want %q", got.configPath, tt.wantPath)
			}
			if got.showHelp != tt.wantHelp {
				t.Fatalf("showHelp = %v, want %v", got.showHelp, tt.wantHelp)
			}
			if got.showVersion != tt.wantVer {
				t.Fatalf("showVersion = %v, want %v", got.showVersion, tt.wantVer)
			}
		})
	}
}
