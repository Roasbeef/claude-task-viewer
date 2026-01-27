package taskviewer

import (
	"bufio"
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ClaudeInstance represents a running Claude Code process.
type ClaudeInstance struct {
	PID         int       `json:"pid"`
	WorkingDir  string    `json:"workingDir"`
	SessionID   string    `json:"sessionId"`
	ProjectName string    `json:"projectName"`
	StartTime   time.Time `json:"startTime"`
	Uptime      string    `json:"uptime"`
	HasTasks    bool      `json:"hasTasks"`
	TaskCount   int       `json:"taskCount"`
}

// InstanceTracker detects running Claude Code instances.
type InstanceTracker struct {
	projectIndexer *ProjectIndexer
}

// NewInstanceTracker creates a new instance tracker.
func NewInstanceTracker(projectIndexer *ProjectIndexer) *InstanceTracker {
	return &InstanceTracker{
		projectIndexer: projectIndexer,
	}
}

// ListRunningInstances finds all running Claude Code processes.
func (it *InstanceTracker) ListRunningInstances() ([]ClaudeInstance, error) {
	// Find claude processes using pgrep and ps.
	pids, err := it.findClaudePIDs()
	if err != nil || len(pids) == 0 {
		return nil, nil
	}

	var instances []ClaudeInstance
	for _, pid := range pids {
		instance, err := it.getInstanceInfo(pid)
		if err != nil {
			continue
		}

		// Enrich with project indexer data.
		it.enrichInstance(instance)
		instances = append(instances, *instance)
	}

	return instances, nil
}

// findClaudePIDs uses pgrep to find Claude Code process IDs.
func (it *InstanceTracker) findClaudePIDs() ([]int, error) {
	// Look for the main claude node process.
	// Claude Code runs as a node process with "claude" in the command.
	cmd := exec.Command("pgrep", "-f", "claude.*node")
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, nil
			}
		}
		// Fall back to searching for "claude" in process list.
		return it.findClaudePIDsFallback()
	}

	return it.parsePIDs(output), nil
}

// findClaudePIDsFallback uses ps to find Claude processes.
func (it *InstanceTracker) findClaudePIDsFallback() ([]int, error) {
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		// Look for node processes running claude.
		if strings.Contains(line, "node") &&
			strings.Contains(line, "claude") &&
			!strings.Contains(line, "grep") {

			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if pid, err := strconv.Atoi(fields[1]); err == nil {
					pids = append(pids, pid)
				}
			}
		}
	}

	return pids, nil
}

// parsePIDs parses newline-separated PIDs from command output.
func (it *InstanceTracker) parsePIDs(output []byte) []int {
	var pids []int
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if pid, err := strconv.Atoi(line); err == nil {
			pids = append(pids, pid)
		}
	}
	return pids
}

// getInstanceInfo retrieves detailed info about a Claude process.
func (it *InstanceTracker) getInstanceInfo(pid int) (*ClaudeInstance, error) {
	instance := &ClaudeInstance{
		PID: pid,
	}

	// Get working directory using lsof.
	cwd, err := it.getProcessCWD(pid)
	if err == nil && cwd != "" {
		instance.WorkingDir = cwd
	}

	// Get process start time using ps.
	startTime, uptime, err := it.getProcessTimes(pid)
	if err == nil {
		instance.StartTime = startTime
		instance.Uptime = uptime
	}

	return instance, nil
}

// getProcessCWD gets the current working directory of a process.
func (it *InstanceTracker) getProcessCWD(pid int) (string, error) {
	// Use lsof to find the cwd.
	cmd := exec.Command("lsof", "-p", strconv.Itoa(pid), "-Fn")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// Parse lsof output for cwd (field type 'cwd').
	scanner := bufio.NewScanner(bytes.NewReader(output))
	foundCwd := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "fcwd" {
			foundCwd = true
			continue
		}
		if foundCwd && strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}

	return "", nil
}

// getProcessTimes gets the start time and uptime of a process.
func (it *InstanceTracker) getProcessTimes(pid int) (time.Time, string, error) {
	// Use ps to get elapsed time.
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etime=")
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, "", err
	}

	etimeStr := strings.TrimSpace(string(output))
	uptime := it.formatUptime(etimeStr)

	// Calculate start time from elapsed time.
	duration := it.parseEtime(etimeStr)
	startTime := time.Now().Add(-duration)

	return startTime, uptime, nil
}

// parseEtime parses ps etime format: [[DD-]HH:]MM:SS
func (it *InstanceTracker) parseEtime(etime string) time.Duration {
	etime = strings.TrimSpace(etime)

	var days, hours, minutes, seconds int

	// Check for days component.
	if strings.Contains(etime, "-") {
		parts := strings.SplitN(etime, "-", 2)
		days, _ = strconv.Atoi(parts[0])
		etime = parts[1]
	}

	parts := strings.Split(etime, ":")
	switch len(parts) {
	case 3: // HH:MM:SS
		hours, _ = strconv.Atoi(parts[0])
		minutes, _ = strconv.Atoi(parts[1])
		seconds, _ = strconv.Atoi(parts[2])
	case 2: // MM:SS
		minutes, _ = strconv.Atoi(parts[0])
		seconds, _ = strconv.Atoi(parts[1])
	case 1: // SS
		seconds, _ = strconv.Atoi(parts[0])
	}

	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second
}

// formatUptime converts etime to human-readable format.
func (it *InstanceTracker) formatUptime(etime string) string {
	duration := it.parseEtime(etime)

	if duration < time.Minute {
		return "just now"
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 min"
		}
		return strconv.Itoa(minutes) + " mins"
	}

	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return strconv.Itoa(hours) + " hours"
	}

	days := int(duration.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return strconv.Itoa(days) + " days"
}

// enrichInstance adds project and task data to an instance.
func (it *InstanceTracker) enrichInstance(instance *ClaudeInstance) {
	if instance.WorkingDir == "" || it.projectIndexer == nil {
		return
	}

	// Extract project name from working directory.
	instance.ProjectName = extractProjectName(
		strings.ReplaceAll(instance.WorkingDir, "/", "-"),
	)

	// Try to find active session for this project.
	activeLists, err := it.projectIndexer.ListActiveTaskLists()
	if err != nil {
		return
	}

	// Match by project path.
	for _, active := range activeLists {
		if active.ProjectPath != "" &&
			strings.Contains(instance.WorkingDir, active.ProjectName) {

			instance.SessionID = active.SessionID
			instance.TaskCount = active.TaskCount
			instance.HasTasks = active.TaskCount > 0
			break
		}
	}
}
