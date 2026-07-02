package audit

import (
	"testing"
)

const goroutineCaptureGoSrc = `package sample

import "fmt"

func Bad() {
	for i := 0; i < 10; i++ {
		go func() { fmt.Println(i) }()
	}
}
`

const goroutineNoCaptureGoSrc = `package sample

import "fmt"

func Good() {
	for i := 0; i < 10; i++ {
		i := i
		go func() { fmt.Println(i) }()
	}
}
`

const mutexWithoutDeferGoSrc = `package sample

import "sync"

type S struct{ mu sync.Mutex }

func (s *S) Bad() {
	s.mu.Lock()
	// no defer Unlock
}
`

const mutexWithDeferGoSrc = `package sample

import "sync"

type S struct{ mu sync.Mutex }

func (s *S) Good() {
	s.mu.Lock()
	defer s.mu.Unlock()
}
`

func TestConcurrencyPass_GoroutineCapture(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "cap.go", goroutineCaptureGoSrc)
	ctx := AuditContext{
		Imports: []string{"sync"},
		Files:   []string{path},
	}
	findings := RunConcurrencyPass(ctx)
	found := false
	for _, f := range findings {
		if f.Type == "concurrency_goroutine_loop_capture" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected goroutine_loop_capture finding, got: %v", findings)
	}
}

func TestConcurrencyPass_GoroutineNoCapture(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "nocap.go", goroutineNoCaptureGoSrc)
	ctx := AuditContext{
		Imports: []string{"sync"},
		Files:   []string{path},
	}
	findings := RunConcurrencyPass(ctx)
	for _, f := range findings {
		if f.Type == "concurrency_goroutine_loop_capture" {
			t.Errorf("unexpected capture finding: %s", f.Description)
		}
	}
}

func TestConcurrencyPass_MutexWithoutDefer(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "mut.go", mutexWithoutDeferGoSrc)
	ctx := AuditContext{
		Imports: []string{"sync"},
		Files:   []string{path},
	}
	findings := RunConcurrencyPass(ctx)
	found := false
	for _, f := range findings {
		if f.Type == "concurrency_mutex_no_defer_unlock" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mutex_no_defer_unlock finding, got: %v", findings)
	}
}

func TestConcurrencyPass_MutexWithDefer(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "good.go", mutexWithDeferGoSrc)
	ctx := AuditContext{
		Imports: []string{"sync"},
		Files:   []string{path},
	}
	findings := RunConcurrencyPass(ctx)
	for _, f := range findings {
		if f.Type == "concurrency_mutex_no_defer_unlock" {
			t.Errorf("unexpected mutex_no_defer_unlock finding: %s", f.Description)
		}
	}
}

func TestConcurrencyPass_NoImports(t *testing.T) {
	ctx := AuditContext{}
	findings := RunConcurrencyPass(ctx)
	if len(findings) != 0 {
		t.Errorf("expected no findings without imports, got %d", len(findings))
	}
}

func TestConcurrencyPass_ImportHeuristicRetained(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "plain.go", "package sample\n")
	ctx := AuditContext{
		Imports: []string{"sync"},
		Files:   []string{path},
	}
	findings := RunConcurrencyPass(ctx)
	found := false
	for _, f := range findings {
		if f.Type == "concurrency_primitive_usage" {
			found = true
		}
	}
	if !found {
		t.Error("expected concurrency_primitive_usage finding to be retained")
	}
}
