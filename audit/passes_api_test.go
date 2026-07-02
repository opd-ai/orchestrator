package audit

import (
	"testing"
)

func TestUndocumentedExportFinding_NoDoc(t *testing.T) {
	sym := SymbolInfo{Name: "Foo", Kind: "func", Exported: true, Package: "pkg", HasDoc: false}
	f, ok := undocumentedExportFinding(sym)
	if !ok {
		t.Fatal("expected a finding for undocumented export")
	}
	if f.Type != "api_undocumented_export" {
		t.Errorf("unexpected type: %s", f.Type)
	}
}

func TestUndocumentedExportFinding_WithDoc(t *testing.T) {
	sym := SymbolInfo{Name: "Bar", Kind: "func", Exported: true, Package: "pkg", HasDoc: true}
	_, ok := undocumentedExportFinding(sym)
	if ok {
		t.Error("expected no finding for documented export")
	}
}

func TestInterfaceDriftFinding_NoConcrete(t *testing.T) {
	iface := SymbolInfo{
		Name:    "Runner",
		Kind:    "interface",
		Package: "pkg",
		Methods: []string{"Run", "Stop"},
	}
	exports := []SymbolInfo{iface}
	f, ok := interfaceDriftFinding(iface, exports)
	if !ok {
		t.Fatal("expected interface drift finding when no implementor present")
	}
	if f.Type != "api_interface_drift" {
		t.Errorf("unexpected type: %s", f.Type)
	}
}

func TestInterfaceDriftFinding_WithConcrete(t *testing.T) {
	iface := SymbolInfo{Name: "Runner", Kind: "interface", Package: "pkg", Methods: []string{"Run"}}
	runMethod := SymbolInfo{Name: "Run", Kind: "method", Receiver: "*Worker", Package: "pkg"}
	exports := []SymbolInfo{iface, runMethod}
	_, ok := interfaceDriftFinding(iface, exports)
	if ok {
		t.Error("expected no drift finding when concrete implementor is present")
	}
}

func TestInterfaceDriftFinding_EmptyInterface(t *testing.T) {
	iface := SymbolInfo{Name: "Any", Kind: "interface", Package: "pkg", Methods: nil}
	_, ok := interfaceDriftFinding(iface, []SymbolInfo{iface})
	if ok {
		t.Error("empty interface should not produce a drift finding")
	}
}

func TestRunAPIPass_IncludesNewChecks(t *testing.T) {
	ctx := AuditContext{
		Exports: []SymbolInfo{
			{Name: "Doer", Kind: "interface", Package: "pkg", Methods: []string{"Do"}, HasDoc: false},
		},
	}
	findings := RunAPIPass(ctx)
	typesSeen := make(map[string]bool)
	for _, f := range findings {
		typesSeen[f.Type] = true
	}
	if !typesSeen["api_undocumented_export"] {
		t.Error("expected api_undocumented_export finding")
	}
	if !typesSeen["api_interface_drift"] {
		t.Error("expected api_interface_drift finding")
	}
}
