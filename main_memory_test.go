package main

import "testing"

func TestPromptWithMemory(t *testing.T) {
	original := plannerMemoryContext
	t.Cleanup(func() {
		plannerMemoryContext = original
	})

	plannerMemoryContext = "Recent adaptive metrics"

	prompt := promptWithMemory("Implement the task")
	if prompt != "Recent adaptive metrics\nImplement the task" {
		t.Fatalf("unexpected prompt: %q", prompt)
	}
}

func TestPromptWithMemoryWithoutContext(t *testing.T) {
	original := plannerMemoryContext
	t.Cleanup(func() {
		plannerMemoryContext = original
	})

	plannerMemoryContext = ""

	prompt := promptWithMemory("Implement the task")
	if prompt != "Implement the task" {
		t.Fatalf("unexpected prompt without memory: %q", prompt)
	}
}

func TestCompressPromptPreservesStructure(t *testing.T) {
	input := "  line 1\n\nline 1\n\tline 2\n"
	if got := compressPrompt(input); got != "line 1\n\nline 1\n\tline 2" {
		t.Fatalf("compressPrompt() = %q, want %q", got, "line 1\n\nline 1\n\tline 2")
	}
}
