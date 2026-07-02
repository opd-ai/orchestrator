package main

func main() {
	parseFlags()
	openLogFile()

	if auditMode {
		runAuditMode()
		return
	}

	runExecutionMode()
}
