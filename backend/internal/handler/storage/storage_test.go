package storage

import (
	"testing"

	"github.com/baihua19941101/cdnManage/internal/config"
)

func TestClampUploadFileParallelism(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{name: "zero uses default", input: 0, want: config.DefaultUploadFileParallelism},
		{name: "below minimum clamps", input: -1, want: config.MinUploadFileParallelism},
		{name: "above maximum clamps", input: config.MaxUploadFileParallelism + 1, want: config.MaxUploadFileParallelism},
		{name: "in range keeps value", input: 7, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampUploadFileParallelism(tt.input)
			if got != tt.want {
				t.Fatalf("unexpected parallelism: got %d want %d", got, tt.want)
			}
		})
	}
}

func TestNewHandlerAppliesFileParallelismClamp(t *testing.T) {
	handler := NewHandler(nil, nil, 0, 0, config.MaxUploadFileParallelism+10)
	if handler.fileParallelism != config.MaxUploadFileParallelism {
		t.Fatalf("unexpected file parallelism: got %d want %d", handler.fileParallelism, config.MaxUploadFileParallelism)
	}
}
