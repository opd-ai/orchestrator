package main

func main() {
	parseFlags()

	if auditMode {
		runAuditMode()
		return
	}

	runExecutionMode()
}
