package sandbox

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLogOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    LogOptions
		wantErr bool
	}{
		{
			name:    "default opts valid",
			opts:    LogOptions{Tail: -1, Head: -1},
			wantErr: false,
		},
		{
			name:    "follow and timestamps valid",
			opts:    LogOptions{Follow: true, Timestamps: true, Tail: -1, Head: -1},
			wantErr: false,
		},
		{
			name:    "follow with tail valid",
			opts:    LogOptions{Follow: true, Tail: 10, Head: -1},
			wantErr: false,
		},
		{
			name:    "follow with head valid",
			opts:    LogOptions{Follow: true, Tail: -1, Head: 5},
			wantErr: false,
		},
		{
			name:    "tail only valid",
			opts:    LogOptions{Tail: 10, Head: -1},
			wantErr: false,
		},
		{
			name:    "tail zero valid",
			opts:    LogOptions{Tail: 0, Head: -1},
			wantErr: false,
		},
		{
			name:    "head only valid",
			opts:    LogOptions{Tail: -1, Head: 5},
			wantErr: false,
		},
		{
			name:    "head zero valid",
			opts:    LogOptions{Tail: -1, Head: 0},
			wantErr: false,
		},
		{
			name:    "both tail and head invalid",
			opts:    LogOptions{Tail: 10, Head: 5},
			wantErr: true,
		},
		{
			name:    "negative tail invalid",
			opts:    LogOptions{Tail: -5, Head: -1},
			wantErr: true,
		},
		{
			name:    "negative head invalid",
			opts:    LogOptions{Tail: -1, Head: -3},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LogOptions.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCopyHead(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		head     int64
		expected string
	}{
		{
			name:     "head 0 returns nothing",
			input:    "line1\nline2\nline3\n",
			head:     0,
			expected: "",
		},
		{
			name:     "head 2 out of 4 lines",
			input:    "line1\nline2\nline3\nline4\n",
			head:     2,
			expected: "line1\nline2\n",
		},
		{
			name:     "head larger than stream line count",
			input:    "line1\nline2\n",
			head:     10,
			expected: "line1\nline2\n",
		},
		{
			name:     "empty input",
			input:    "",
			head:     5,
			expected: "",
		},
		{
			name:     "stream without trailing newline",
			input:    "alpha\nbeta",
			head:     2,
			expected: "alpha\nbeta",
		},
		{
			name:     "head 1 with trailing newline",
			input:    "first\nsecond\nthird\n",
			head:     1,
			expected: "first\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			var out bytes.Buffer
			err := CopyHead(r, &out, tt.head)
			if err != nil {
				t.Fatalf("CopyHead() unexpected error: %v", err)
			}
			if out.String() != tt.expected {
				t.Fatalf("CopyHead() got %q, want %q", out.String(), tt.expected)
			}
		})
	}
}

func TestCopyHeadStreaming(t *testing.T) {
	pr, pw := io.Pipe()
	done := make(chan struct{})
	var out bytes.Buffer

	go func() {
		defer close(done)
		err := CopyHead(pr, &out, 3)
		if err != nil {
			t.Errorf("CopyHead failed: %v", err)
		}
	}()

	go func() {
		defer pw.Close()
		for i := 1; i <= 10; i++ {
			fmt.Fprintf(pw, "stream-line-%d\n", i)
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		expected := "stream-line-1\nstream-line-2\nstream-line-3\n"
		if out.String() != expected {
			t.Fatalf("CopyHead got %q, want %q", out.String(), expected)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CopyHead timed out on streaming input")
	}
}
