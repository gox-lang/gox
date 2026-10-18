package gox

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestMultipartFilesParseWithFetchFormData(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for Fetch FormData integration")
	}
	for _, write := range []struct {
		name string
		fn   func(*Context, []File) error
	}{
		{"action", WriteActionFiles}, {"page", WritePageFiles},
	} {
		t.Run(write.name, func(t *testing.T) {
			for _, names := range [][]string{nil, {"first.txt", "second.bin"}, {"reporte año.txt", "empty-🚀.txt", `quoted".txt`}} {
				files := make([]File, 0, len(names))
				for _, name := range names {
					file, err := NewFile(FileConfig{Name: name, Type: "text/plain", Content: strings.NewReader("payload")})
					if err != nil {
						t.Fatal(err)
					}
					files = append(files, file)
				}
				response := NewResponse()
				writer := &pageFileTestWriter{}
				if err := write.fn(&Context{Response: response, Writer: writer}, files); err != nil {
					t.Fatal(err)
				}
				if names == nil {
					names = []string{}
				}
				input, err := json.Marshal(map[string]any{"type": responseHeaderValue(&response.Headers, "Content-Type"), "body": writer.body.String(), "names": names})
				if err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command(node, "--input-type=module", "-e", `
import assert from 'node:assert/strict';
let input = ''; for await (const chunk of process.stdin) input += chunk;
const data = JSON.parse(input);
const form = await new Response(data.body, {headers:{'Content-Type':data.type}}).formData();
const files = form.getAll('files');
assert.deepEqual(files.map(file => file.name), data.names);
for (const file of files) assert.equal(await file.text(), 'payload');
`)
				cmd.Stdin = strings.NewReader(string(input))
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("Fetch FormData failed for %q: %v\n%s", names, err, output)
				}
			}
		})
	}
}
