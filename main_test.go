package main

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func captureStdout(f func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()

	w.Close()

	os.Stdout = oldStdout
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func Test_iterateTarEntries(t *testing.T) {
	type args struct {
		tarPath        string
		compressorType string
		op             tarOp
	}
	tests := []struct {
		name           string
		args           args
		expectedOutput string
		wantErr        bool
	}{
		{
			name: "lists files in files.tar",
			args: args{
				tarPath:        "testdata/files.tar",
				compressorType: "",
				op:             listTarFiles,
			},
			expectedOutput: "file1.txt\nfile2.txt\nfile3.txt\n",
			wantErr:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(func() {
				if err := iterateTarEntries(tt.args.tarPath, tt.args.compressorType, tt.args.op); (err != nil) != tt.wantErr {

					t.Errorf("iterateTarEntries() error = %v, wantErr %v", err, tt.wantErr)
				}
			})

			if output != tt.expectedOutput {
				t.Errorf("received: %s, expected: %s", output, tt.expectedOutput)

			}
		})
	}
}
