package basispoints

import (
	"reflect"
	"testing"
)

func TestBPSHistoryMessageIDCompatibility(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, attributed := range []bool{false, true} {
			for _, id := range []string{"item_3bcecdc600ec926478685c43", "fc_foreign", "", "msg_valid"} {
				item := object{"role": "assistant", "id": id, "phase": "commentary", "status": "completed", "content": []any{object{"type": "output_text", "text": "Keep this history.", "annotations": []any{}}}}
				if typed {
					item["type"] = "message"
				}
				if attributed {
					item["author"] = "/root"
				}
				source := object{"model": "gpt-6-astra", "input": []any{item}}
				prepared, _ := mustPrepare(t, source, "message-id", nil)
				items := mustTestValue[[]any](t, prepared["input"])
				got := mustTestValue[object](t, items[len(items)-1])
				if id == "msg_valid" {
					if got["id"] != id {
						t.Fatalf("valid ID changed: %v", got)
					}
				} else if _, exists := got["id"]; exists {
					t.Fatalf("invalid message ID forwarded: %q (typed=%v attributed=%v)", id, typed, attributed)
				}
				for _, key := range []string{"role", "phase", "status"} {
					if got[key] != item[key] {
						t.Fatalf("%s changed", key)
					}
				}
				parts := mustTestValue[[]any](t, got["content"])
				original := mustTestValue[[]any](t, item["content"])
				if !reflect.DeepEqual(parts[len(parts)-1], original[0]) {
					t.Fatal("content changed")
				}
				if item["id"] != id {
					t.Fatal("source message mutated")
				}
			}
		}
	}
}

func TestBPSMessageIDNormalizationLeavesToolAndReasoningIDsAlone(t *testing.T) {
	for _, kind := range []string{"function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "reasoning", "item_reference"} {
		item := object{"type": kind, "id": "item_original", "call_id": "call_original"}
		got, err := normalizeHistoryMessage(item, 0)
		if err != nil || !reflect.DeepEqual(got, item) {
			t.Fatalf("%s changed: %v %v", kind, got, err)
		}
	}
}

func TestBPSMessageIDNormalizationDoesNotMutateSharedHistory(t *testing.T) {
	original := object{"type": "message", "id": "item_original", "role": "assistant", "content": "Keep history"}
	cleaned, err := normalizeHistoryMessage(original, 0)
	if err != nil {
		t.Fatal(err)
	}
	if original["id"] != "item_original" {
		t.Fatal("shared source history was mutated")
	}
	if _, exists := cleaned["id"]; exists {
		t.Fatal("foreign ID was forwarded")
	}
	again, err := normalizeHistoryMessage(cleaned, 0)
	if err != nil || !reflect.DeepEqual(again, cleaned) {
		t.Fatal("normalization was not idempotent")
	}
}
