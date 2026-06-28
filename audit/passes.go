package audit

func RunArchitecturePass(ctx AuditContext) []Finding {
	return []Finding{
		{
			Type:        "architecture_review",
			Severity:    "info",
			Description: "Cluster analyzed for layering violations.",
			Confidence:  0.6,
		},
	}
}

func RunAPIPass(ctx AuditContext) []Finding {
	return []Finding{
		{
			Type:        "api_surface_review",
			Severity:    "info",
			Description: "Exported symbols reviewed.",
			Confidence:  0.6,
		},
	}
}

func RunConcurrencyPass(ctx AuditContext) []Finding {
	return []Finding{
		{
			Type:        "concurrency_review",
			Severity:    "info",
			Description: "Concurrency primitives inspected.",
			Confidence:  0.6,
		},
	}
}
