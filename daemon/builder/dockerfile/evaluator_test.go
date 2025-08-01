package dockerfile

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/go-archive"
	"github.com/moby/moby/v2/daemon/builder/remotecontext"
	"github.com/moby/sys/reexec"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type dispatchTestCase struct {
	name, expectedError string
	cmd                 instructions.Command
	files               map[string]string
}

func TestMain(m *testing.M) {
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}

func TestDispatch(t *testing.T) {
	if runtime.GOOS != "windows" {
		skip.If(t, os.Getuid() != 0, "skipping test that requires root")
	}
	testCases := []dispatchTestCase{
		{
			name: "ADD multiple files to file",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"file1.txt", "file2.txt"},
				DestPath:    "test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "Wildcard ADD multiple files to file",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"file*.txt"},
				DestPath:    "test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "COPY multiple files to file",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"file1.txt", "file2.txt"},
				DestPath:    "test",
			}},
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"file1.txt": "test1", "file2.txt": "test2"},
		},
		{
			name: "ADD multiple files to file with whitespace",
			cmd: &instructions.AddCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"test file1.txt", "test file2.txt"},
				DestPath:    "test",
			}},
			expectedError: "When using ADD with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name: "COPY multiple files to file with whitespace",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"test file1.txt", "test file2.txt"},
				DestPath:    "test",
			}},
			expectedError: "When using COPY with more than one source file, the destination must be a directory and end with a /",
			files:         map[string]string{"test file1.txt": "test1", "test file2.txt": "test2"},
		},
		{
			name: "COPY wildcard no files",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"file*.txt"},
				DestPath:    "/tmp/",
			}},
			expectedError: "COPY failed: no source files were specified",
			files:         nil,
		},
		{
			name: "COPY url",
			cmd: &instructions.CopyCommand{SourcesAndDest: instructions.SourcesAndDest{
				SourcePaths: []string{"https://example.com/index.html"},
				DestPath:    "/",
			}},
			expectedError: "source can't be a URL for COPY",
			files:         nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contextDir := t.TempDir()

			for filename, content := range tc.files {
				createTestTempFile(t, contextDir, filename, content, 0o777)
			}

			tarStream, err := archive.Tar(contextDir, archive.Uncompressed)
			if err != nil {
				t.Fatalf("Error when creating tar stream: %s", err)
			}

			defer func() {
				if err = tarStream.Close(); err != nil {
					t.Fatalf("Error when closing tar stream: %s", err)
				}
			}()

			buildContext, err := remotecontext.FromArchive(tarStream)
			if err != nil {
				t.Fatalf("Error when creating tar context: %s", err)
			}

			defer func() {
				if err = buildContext.Close(); err != nil {
					t.Fatalf("Error when closing tar context: %s", err)
				}
			}()

			b := newBuilderWithMockBackend(t)
			sb := newDispatchRequest(b, '`', buildContext, NewBuildArgs(make(map[string]*string)), newStagesBuildResults())
			err = dispatch(context.TODO(), sb, tc.cmd)
			assert.Check(t, is.ErrorContains(err, tc.expectedError))
		})
	}
}
