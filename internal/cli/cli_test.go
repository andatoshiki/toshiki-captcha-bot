package cli

import "testing"

func TestParseCLIArgs(t *testing.T) {
	t.Parallel()

	const defaultConfigPath = "config.yaml"

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

			got, err := ParseArgs(tt.args, defaultConfigPath)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("parseCLIArgs returned error: %v", err)
			}
			if got.ConfigPath != tt.wantPath {
				t.Fatalf("configPath = %q, want %q", got.ConfigPath, tt.wantPath)
			}
			if got.ShowHelp != tt.wantHelp {
				t.Fatalf("showHelp = %v, want %v", got.ShowHelp, tt.wantHelp)
			}
			if got.ShowVersion != tt.wantVer {
				t.Fatalf("showVersion = %v, want %v", got.ShowVersion, tt.wantVer)
			}
		})
	}
}
