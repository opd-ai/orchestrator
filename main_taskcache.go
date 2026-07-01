package main

import (
	"encoding/json"
	"os"
)

const taskCacheFile = "task_cache.json"

// loadTaskCache reads the on-disk diff cache ({task_hash -> diff}) if it exists,
// returning an empty map on any error so callers never deal with nil.
func loadTaskCache() map[string]string {
	data, err := os.ReadFile(taskCacheFile)
	if err != nil {
		return map[string]string{}
	}
	var cache map[string]string
	if err := json.Unmarshal(data, &cache); err != nil {
		return map[string]string{}
	}
	return cache
}

// saveTaskCache persists the diff cache to disk. Errors are logged but not fatal.
func saveTaskCache(cache map[string]string) {
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		logError("task_cache_encode_failed", "", err.Error())
		return
	}
	if err := os.WriteFile(taskCacheFile, b, 0o644); err != nil {
		logError("task_cache_write_failed", "", err.Error())
	}
}

// cachedDiff returns the previously successful diff for task.Hash, or "" if not cached.
func cachedDiff(cache map[string]string, task *Task) string {
	if task.Hash == "" {
		return ""
	}
	return cache[task.Hash]
}

// cacheTaskResult stores a successfully applied diff keyed by task hash.
func cacheTaskResult(cache map[string]string, task *Task, diff string) {
	if task.Hash == "" || diff == "" {
		return
	}
	cache[task.Hash] = diff
}
