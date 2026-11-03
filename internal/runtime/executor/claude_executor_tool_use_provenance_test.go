package executor

import (
	"bytes"
	"testing"

	"github.com/MoeForge/moego/ojson"
)

// referenceStripClaudeToolUseProvenanceFieldsDoc is
// stripClaudeToolUseProvenanceFieldsDoc before it consulted the block it is
// handed (upstream v8.0.53): every field is looked up by path on doc.
func referenceStripClaudeToolUseProvenanceFieldsDoc(doc *ojson.Doc, partPath string) bool {
	changed := false
	for _, relPath := range []string{"signature", "thoughtSignature", "thought_signature", "extra_content.google.thought_signature", "model"} {
		sigPath := partPath + "." + relPath
		if !doc.Get(sigPath).Exists() {
			continue
		}
		if doc.Delete(sigPath) == nil {
			changed = true
		}
	}
	for _, objPath := range []string{partPath + ".extra_content.google", partPath + ".extra_content"} {
		res := doc.Get(objPath)
		if res.Exists() && res.IsObject() && len(res.Map()) == 0 {
			if doc.Delete(objPath) == nil {
				changed = true
			}
		}
	}
	return changed
}

func TestStripClaudeToolUseProvenanceFieldsMatchesReference(t *testing.T) {
	blocks := []string{
		`{"type":"tool_use","id":"a","name":"n","input":{}}`,
		`{"type":"tool_use","id":"b","name":"n","input":{"signature":"x","model":"m","extra_content":{}}}`,
		`{"type":"tool_use","id":"c","name":"n","input":{},"signature":"s"}`,
		`{"type":"tool_use","id":"d","name":"n","input":{},"thoughtSignature":"s","model":"gemini-3"}`,
		`{"type":"tool_use","id":"e","name":"n","input":{},"thought_signature":"s"}`,
		`{"type":"tool_use","id":"f","name":"n","input":{},"extra_content":{"google":{"thought_signature":"s"}}}`,
		`{"type":"tool_use","id":"g","name":"n","input":{},"extra_content":{"google":{"thought_signature":"s","other":1}}}`,
		`{"type":"tool_use","id":"h","name":"n","input":{},"extra_content":{}}`,
		`{"type":"tool_use","id":"i","name":"n","input":{},"extra_content":{"google":{},"x":2}}`,
		`{"type":"tool_use","id":"j","name":"n","input":{},"extra_content":"not an object"}`,
		`{"type":"tool_use","id":"k","name":"n","input":{},"model":null}`,
	}
	const partPath = "messages.1.content.1"
	for _, block := range blocks {
		body := []byte(`{"model":"claude-opus-5-5","messages":[{"role":"user","content":"hi"},` +
			`{"role":"assistant","content":[{"type":"text","text":"t"},` + block + `]}]}`)
		got := ojson.NewDoc(body)
		gotChanged := stripClaudeToolUseProvenanceFieldsDoc(got, partPath, ojson.GetBytes(body, partPath))
		want := ojson.NewDoc(body)
		wantChanged := referenceStripClaudeToolUseProvenanceFieldsDoc(want, partPath)
		if gotChanged != wantChanged || !bytes.Equal(got.Bytes(), want.Bytes()) {
			t.Fatalf("block %s: changed=%v body=%s, reference changed=%v body=%s", block, gotChanged, got.Bytes(), wantChanged, want.Bytes())
		}
	}
}
