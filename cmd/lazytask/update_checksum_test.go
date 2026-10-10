package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestExpectedChecksum(t *testing.T) {
	file := []byte("aaa  lazytask_1.0.0_linux_amd64.tar.gz\nBBB *lazytask_1.0.0_darwin_arm64.tar.gz\n")
	cases := []struct {
		name    string
		archive string
		want    string
		wantErr bool
	}{
		{"found", "lazytask_1.0.0_linux_amd64.tar.gz", "aaa", false},
		{"binary-mode marker and case", "lazytask_1.0.0_darwin_arm64.tar.gz", "bbb", false},
		{"missing", "lazytask_1.0.0_linux_arm64.tar.gz", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := expectedChecksum(file, c.archive)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("hello")
	sum := sha256.Sum256(data)
	good := hex.EncodeToString(sum[:])

	if err := verifyChecksum(data, good); err != nil {
		t.Errorf("valid checksum rejected: %v", err)
	}
	if err := verifyChecksum([]byte("tampered"), good); err == nil {
		t.Error("tampered data accepted")
	}
}
