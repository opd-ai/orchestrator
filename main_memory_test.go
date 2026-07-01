package main

import "testing"

func TestPromptWithMemory(t *testing.T) {
	original := plannerMemoryContext
	t.Cleanup(func() {
		plannerMemoryContext = original
	})

	plannerMemoryContext = "Recent adaptive metrics"

	prompt := promptWithMemory("Implement the task")
	if prompt != "Recent adaptive metrics\n\nImplement the task" {
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
