package executor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

func TestClaudeFable52ReplayAndPayloadPrecedence(t *testing.T) {
	for _, model := range []string{"claude-fable-5-2", "claude-mythos-5-2"} {
		for _, stream := range []bool{false, true} {
			for _, policy := range []string{"default", "caller", "override", "filter"} {
				t.Run(fmt.Sprintf("%s/stream=%v/%s", model, stream, policy), func(t *testing.T) {
					cfg := &config.Config{}
					doc := ojson.NewDoc(preservedThinkingReplayPayload(model))
					want := "drop_block"
					switch policy {
					case "caller":
						_ = doc.Set("thinking.block_binding.prefix_mismatch_behavior", "error")
						want = "error"
					case "override":
						cfg.Payload.Override = []config.PayloadRule{{Models: []config.PayloadModelRule{{Name: model}}, Params: map[string]any{"thinking.block_binding.prefix_mismatch_behavior": "error"}}}
						want = "error"
					case "filter":
						cfg.Payload.Filter = []config.PayloadFilterRule{{Models: []config.PayloadModelRule{{Name: model}}, Params: []string{"thinking.block_binding"}}}
						want = ""
					}
					body, headers := runClaudeExecutorCapture(t, cfg, "", stream, model, doc.Bytes())
					if got := ojson.GetBytes(body, "thinking.block_binding.prefix_mismatch_behavior").String(); got != want {
						t.Fatalf("binding = %q, want %q; body=%s", got, want, body)
					}
					if want != "" && !strings.Contains(headers.Get("Anthropic-Beta"), claudeThinkingBindingBeta) {
						t.Fatalf("missing binding beta: %v", headers)
					}
					if !strings.Contains(string(body), preservedThinkingTestSignature) {
						t.Fatal("signed reasoning was lost before upstream replay")
					}
				})
			}
		}
	}
}

func TestClaudeFable52ThreadAndProgress(t *testing.T) {
	for _, model := range []string{"claude-fable-5-2", "claude-mythos-5-2"} {
		for _, thread := range []string{`{"type":"create"}`, `{"type":"continue","previous_message_id":"msg_1"}`} {
			doc := ojson.NewDocString(`{"model":"` + model + `","thread":` + thread + `,"messages":[{"role":"user","content":"hi"}]}`)
			if !applyClaudePreservedThinkingBindingDoc(doc) || doc.Get("thinking.type").String() != "adaptive" {
				t.Fatalf("%s: thread missing adaptive binding: %s", model, doc.String())
			}
			if !applyClaudeCloakThinkingDisplayDoc(doc, false) || doc.Get("thinking.display").String() != "updates" {
				t.Fatalf("%s: missing progress display: %s", model, doc.String())
			}
			_ = doc.Set("output_config.timing", "enabled")
			betas := claudeCodeCLIBetas(doc.Bytes(), nil, true)
			for _, beta := range []string{claudeThinkingBindingBeta, claudeThinkingDisplayUpdatesBeta, claudePerTurnControlBeta, claudePerTurnTimingBeta} {
				if !strings.Contains(betas, beta) {
					t.Fatalf("%s: missing %s in %s", model, beta, betas)
				}
			}
		}
		if claudeThinkingTypeAcceptsBlockBinding(model, "enabled") {
			t.Fatalf("%s requires adaptive thinking", model)
		}
	}
}
