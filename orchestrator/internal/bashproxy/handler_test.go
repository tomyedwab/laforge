package bashproxy

import (
	"testing"
)

func TestExtractUpdateStatusMessage(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
		wantErr bool
	}{
		{
			name:    "simple message with double quotes",
			command: `update_status '{"message": "Hello world"}'`,
			want:    "Hello world",
			wantErr: false,
		},
		{
			name:    "message with apostrophe - simple format",
			command: `update_status '{"message": "We'll get together"}'`,
			want:    "We'll get together",
			wantErr: false,
		},
		{
			name:    "message with escaped quotes - simple format",
			command: `update_status '{"message": "He said \"hello\""}'`,
			want:    `He said "hello"`,
			wantErr: false,
		},
		{
			name:    "actual Claude Code command with eval - no apostrophe",
			command: `source /root/.claude/shell-snapshots/snapshot-bash-1770707090061-rmx6cc.sh && { shopt -u extglob || setopt NO_EXTENDED_GLOB; } >/dev/null 2>&1 || true && eval "update_status '{\"message\": \"Bug found and fixed\"}'" \< /dev/null && pwd -P >| /tmp/claude-42be-cwd`,
			want:    "Bug found and fixed",
			wantErr: false,
		},
		{
			name:    "actual Claude Code command with apostrophe in message",
			command: `source /root/.claude/shell-snapshots/snapshot-bash-1770707090061-rmx6cc.sh && { shopt -u extglob || setopt NO_EXTENDED_GLOB; } >/dev/null 2>&1 || true && eval "update_status '{\"message\": \"Come out to the coast, we'll get together, have a few laughs...\"}'" \< /dev/null && pwd -P >| /tmp/claude-42be-cwd`,
			want:    "Come out to the coast, we'll get together, have a few laughs...",
			wantErr: false,
		},
		{
			name:    "Die Hard quote 1",
			command: `eval "update_status '{\"message\": \"Yippee-ki-yay, motherf***er!\"}'"`,
			want:    "Yippee-ki-yay, motherf***er!",
			wantErr: false,
		},
		{
			name:    "Die Hard quote 4 with apostrophe",
			command: `eval "update_status '{\"message\": \"Come out to the coast, we'll get together, have a few laughs...\"}'"`,
			want:    "Come out to the coast, we'll get together, have a few laughs...",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractUpdateStatusMessage(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractUpdateStatusMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractUpdateStatusMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
