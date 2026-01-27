package taskviewer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Project represents a Claude Code project with its sessions.
type Project struct {
	// Path is the original filesystem path to the project.
	Path string `json:"path"`

	// Name is the human-readable project name (last path component).
	Name string `json:"name"`

	// Shortname is a URL-friendly short identifier (e.g., "darepo").
	Shortname string `json:"shortname"`

	// Org is the GitHub org or user (e.g., "lightninglabs", "roasbeef").
	Org string `json:"org"`

	// DirName is the sanitized directory name in ~/.claude/projects/.
	DirName string `json:"dirName"`

	// Sessions contains all session entries for this project.
	Sessions []SessionEntry `json:"sessions"`

	// SessionCount is the total number of sessions.
	SessionCount int `json:"sessionCount"`

	// LastModified is the most recent session modification time.
	LastModified time.Time `json:"lastModified"`
}

// SessionEntry represents a single session from sessions-index.json.
type SessionEntry struct {
	SessionID    string    `json:"sessionId"`
	FullPath     string    `json:"fullPath"`
	FileMtime    int64     `json:"fileMtime"`
	FirstPrompt  string    `json:"firstPrompt"`
	Summary      string    `json:"summary"`
	MessageCount int       `json:"messageCount"`
	Created      time.Time `json:"created"`
	Modified     time.Time `json:"modified"`
	GitBranch    string    `json:"gitBranch"`
	ProjectPath  string    `json:"projectPath"`
	IsSidechain  bool      `json:"isSidechain"`
}

// sessionsIndex is the structure of sessions-index.json.
type sessionsIndex struct {
	Version int            `json:"version"`
	Entries []SessionEntry `json:"entries"`
}

// ProjectIndexer scans ~/.claude/projects/ and builds project metadata.
type ProjectIndexer struct {
	projectsDir string
	tasksDir    string
}

// NewProjectIndexer creates a new project indexer.
func NewProjectIndexer(claudeDir string) *ProjectIndexer {
	if claudeDir == "" {
		home, _ := os.UserHomeDir()
		claudeDir = filepath.Join(home, ".claude")
	}

	return &ProjectIndexer{
		projectsDir: filepath.Join(claudeDir, "projects"),
		tasksDir:    filepath.Join(claudeDir, "tasks"),
	}
}

// ListProjects returns all projects with their session metadata.
func (pi *ProjectIndexer) ListProjects() ([]Project, error) {
	entries, err := os.ReadDir(pi.projectsDir)
	if err != nil {
		return nil, err
	}

	var projects []Project
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		project, err := pi.loadProject(entry.Name())
		if err != nil {
			// Skip invalid projects.
			continue
		}

		projects = append(projects, project)
	}

	// Sort by last modified time (most recent first).
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastModified.After(projects[j].LastModified)
	})

	return projects, nil
}

// loadProject reads the sessions-index.json for a project directory.
func (pi *ProjectIndexer) loadProject(dirName string) (Project, error) {
	indexPath := filepath.Join(pi.projectsDir, dirName, "sessions-index.json")

	data, err := os.ReadFile(indexPath)
	if err != nil {
		return Project{}, err
	}

	var idx sessionsIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return Project{}, err
	}

	// Skip projects with no sessions.
	if len(idx.Entries) == 0 {
		return Project{}, os.ErrNotExist
	}

	// Sort sessions by modified time (most recent first).
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Modified.After(idx.Entries[j].Modified)
	})

	// Get the project path from the first entry (most recently modified).
	projectPath := idx.Entries[0].ProjectPath

	// Extract project name, shortname, and org from path.
	name, shortname, org := extractProjectInfo(projectPath)
	if name == "" {
		name = dirName
	}

	// Get most recent modification time (entries already sorted, first is most recent).
	lastModified := idx.Entries[0].Modified

	return Project{
		Path:         projectPath,
		Name:         name,
		Shortname:    shortname,
		Org:          org,
		DirName:      dirName,
		Sessions:     idx.Entries,
		SessionCount: len(idx.Entries),
		LastModified: lastModified,
	}, nil
}

// extractProjectInfo extracts name, shortname, and org from a project path.
// For paths like /Users/roasbeef/gocode/src/github.com/lightninglabs/darepo,
// returns name="darepo", shortname="darepo", org="lightninglabs".
func extractProjectInfo(path string) (name, shortname, org string) {
	// Get the base name.
	name = filepath.Base(path)
	if name == "" || name == "." {
		return "", "", ""
	}
	shortname = name

	// Try to extract org from github.com paths.
	parts := strings.Split(path, string(filepath.Separator))
	for i, part := range parts {
		if part == "github.com" && i+1 < len(parts) {
			org = parts[i+1]
			break
		}
	}

	return name, shortname, org
}

// GetProject returns a single project by its directory name.
func (pi *ProjectIndexer) GetProject(dirName string) (Project, error) {
	return pi.loadProject(dirName)
}

// GetProjectByPath finds a project by its original filesystem path.
func (pi *ProjectIndexer) GetProjectByPath(path string) (Project, error) {
	projects, err := pi.ListProjects()
	if err != nil {
		return Project{}, err
	}

	for _, p := range projects {
		if p.Path == path {
			return p, nil
		}
	}

	return Project{}, os.ErrNotExist
}

// HasTasks checks if a session has any tasks in ~/.claude/tasks/.
func (pi *ProjectIndexer) HasTasks(sessionID string) bool {
	taskDir := filepath.Join(pi.tasksDir, sessionID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return false
	}

	// Check for actual task files (not just .lock).
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			return true
		}
	}

	return false
}

// GetTaskCount returns the number of tasks for a session.
func (pi *ProjectIndexer) GetTaskCount(sessionID string) int {
	taskDir := filepath.Join(pi.tasksDir, sessionID)
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return 0
	}

	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}

	return count
}

// ActiveTaskList represents a task list with active tasks.
type ActiveTaskList struct {
	SessionID   string
	TaskCount   int
	TaskDir     string
	ProjectName string
	ProjectPath string
	Summary     string
	FirstPrompt string
}

// ListActiveTaskLists returns all task lists that have actual task files.
func (pi *ProjectIndexer) ListActiveTaskLists() ([]ActiveTaskList, error) {
	entries, err := os.ReadDir(pi.tasksDir)
	if err != nil {
		return nil, err
	}

	// Build a map of sessionID -> project info by scanning all projects.
	sessionToProject := pi.buildSessionProjectMap()

	var activeLists []ActiveTaskList
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		taskCount := pi.GetTaskCount(sessionID)
		if taskCount > 0 {
			active := ActiveTaskList{
				SessionID: sessionID,
				TaskCount: taskCount,
				TaskDir:   filepath.Join(pi.tasksDir, sessionID),
			}

			// Look up the project for this session.
			if proj, ok := sessionToProject[sessionID]; ok {
				active.ProjectName = proj.name
				active.ProjectPath = proj.path
				active.Summary = proj.summary
				active.FirstPrompt = proj.firstPrompt
			} else if proj, ok := pi.findProjectByJSONL(sessionID); ok {
				// Fallback: search for the JSONL file directly.
				active.ProjectName = proj.name
				active.ProjectPath = proj.path
				active.Summary = proj.summary
				active.FirstPrompt = proj.firstPrompt
			}

			activeLists = append(activeLists, active)
		}
	}

	return activeLists, nil
}

// projectInfo holds minimal project info for session lookups.
type projectInfo struct {
	name        string
	path        string
	summary     string
	firstPrompt string
}

// buildSessionProjectMap scans all projects and builds a map of session IDs
// to their project info.
func (pi *ProjectIndexer) buildSessionProjectMap() map[string]projectInfo {
	result := make(map[string]projectInfo)

	projectDirs, err := os.ReadDir(pi.projectsDir)
	if err != nil {
		return result
	}

	for _, dir := range projectDirs {
		if !dir.IsDir() {
			continue
		}

		indexPath := filepath.Join(
			pi.projectsDir, dir.Name(), "sessions-index.json",
		)
		data, err := os.ReadFile(indexPath)
		if err != nil {
			continue
		}

		var index sessionsIndex
		if err := json.Unmarshal(data, &index); err != nil {
			continue
		}

		// Extract a short project name from the directory name.
		projectName := extractProjectName(dir.Name())

		for _, entry := range index.Entries {
			result[entry.SessionID] = projectInfo{
				name:        projectName,
				path:        dir.Name(),
				summary:     entry.Summary,
				firstPrompt: entry.FirstPrompt,
			}
		}
	}

	return result
}

// extractProjectName extracts a short project name from a sanitized dir name.
// E.g., "-Users-roasbeef-gocode-src-github-com-roasbeef-lnd" -> "lnd"
func extractProjectName(dirName string) string {
	parts := strings.Split(dirName, "-")
	if len(parts) > 0 {
		// Return the last non-empty part.
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				return parts[i]
			}
		}
	}
	return dirName
}

// findProjectByJSONL searches for a session's JSONL file in project directories.
// This is a fallback when the session isn't in sessions-index.json yet.
func (pi *ProjectIndexer) findProjectByJSONL(sessionID string) (projectInfo, bool) {
	projectDirs, err := os.ReadDir(pi.projectsDir)
	if err != nil {
		return projectInfo{}, false
	}

	jsonlName := sessionID + ".jsonl"
	for _, dir := range projectDirs {
		if !dir.IsDir() {
			continue
		}

		jsonlPath := filepath.Join(pi.projectsDir, dir.Name(), jsonlName)
		if _, err := os.Stat(jsonlPath); err == nil {
			return projectInfo{
				name: extractProjectName(dir.Name()),
				path: dir.Name(),
			}, true
		}
	}

	return projectInfo{}, false
}

// ProjectSummary provides a condensed view of a project for listing.
type ProjectSummary struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	DirName      string    `json:"dirName"`
	Shortname    string    `json:"shortname"`
	Org          string    `json:"org"`
	BaseRepo     string    `json:"baseRepo"`
	SessionCount int       `json:"sessionCount"`
	LastModified time.Time `json:"lastModified"`
	LastBranch   string    `json:"lastBranch"`
	LastSummary  string    `json:"lastSummary"`
}

// ProjectGroup groups related projects (worktrees) under a base repo.
type ProjectGroup struct {
	BaseRepo     string           `json:"baseRepo"`
	Org          string           `json:"org"`
	Projects     []ProjectSummary `json:"projects"`
	TotalSessions int             `json:"totalSessions"`
	LastModified time.Time        `json:"lastModified"`
}

// ListProjectSummaries returns a condensed list of all projects.
func (pi *ProjectIndexer) ListProjectSummaries() ([]ProjectSummary, error) {
	projects, err := pi.ListProjects()
	if err != nil {
		return nil, err
	}

	summaries := make([]ProjectSummary, len(projects))
	for i, p := range projects {
		summary := ProjectSummary{
			Name:         p.Name,
			Path:         p.Path,
			DirName:      p.DirName,
			Shortname:    p.Shortname,
			Org:          p.Org,
			SessionCount: p.SessionCount,
			LastModified: p.LastModified,
		}

		if len(p.Sessions) > 0 {
			summary.LastBranch = p.Sessions[0].GitBranch
			summary.LastSummary = p.Sessions[0].Summary
		}

		summaries[i] = summary
	}

	// Compute base repos for grouping.
	computeBaseRepos(summaries)

	return summaries, nil
}

// computeBaseRepos determines the base repo for each project.
// Worktrees follow the pattern: {base-repo}-{suffix} where suffix can be multi-part.
// The algorithm finds the LONGEST matching base repo to handle cases like:
// - darepo (base repo)
// - darepo-client (separate base repo, not worktree of darepo)
// - darepo-client-worktree (worktree of darepo-client)
func computeBaseRepos(summaries []ProjectSummary) {
	// Collect all repo names by org.
	orgRepos := make(map[string][]string)
	for _, s := range summaries {
		if s.Org != "" {
			orgRepos[s.Org] = append(orgRepos[s.Org], s.Name)
		}
	}

	// Sort each org's repos by length (longest first for matching).
	for org := range orgRepos {
		sort.Slice(orgRepos[org], func(i, j int) bool {
			return len(orgRepos[org][i]) > len(orgRepos[org][j])
		})
	}

	// For each summary, find its base repo (longest matching prefix).
	for i := range summaries {
		s := &summaries[i]
		if s.Org == "" {
			s.BaseRepo = s.Name
			continue
		}

		// Find the longest base repo that this name starts with.
		// Repos are sorted longest-first, so first prefix match wins.
		repos := orgRepos[s.Org]
		s.BaseRepo = s.Name // Default to self (is its own base repo).
		for _, candidate := range repos {
			// Skip self - a project can't be a worktree of itself.
			if candidate == s.Name {
				continue
			}
			// Check if this is a worktree (name starts with candidate-).
			if strings.HasPrefix(s.Name, candidate+"-") {
				// Found a matching base (longest first, so first match is best).
				s.BaseRepo = candidate
				break
			}
		}
	}
}

// ListProjectGroups returns projects grouped by base repo.
func (pi *ProjectIndexer) ListProjectGroups() ([]ProjectGroup, error) {
	summaries, err := pi.ListProjectSummaries()
	if err != nil {
		return nil, err
	}

	// Group by org+baseRepo.
	groupMap := make(map[string]*ProjectGroup)
	for _, s := range summaries {
		key := s.Org + "/" + s.BaseRepo
		if g, ok := groupMap[key]; ok {
			g.Projects = append(g.Projects, s)
			g.TotalSessions += s.SessionCount
			if s.LastModified.After(g.LastModified) {
				g.LastModified = s.LastModified
			}
		} else {
			groupMap[key] = &ProjectGroup{
				BaseRepo:      s.BaseRepo,
				Org:           s.Org,
				Projects:      []ProjectSummary{s},
				TotalSessions: s.SessionCount,
				LastModified:  s.LastModified,
			}
		}
	}

	// Convert to slice and sort by last modified.
	groups := make([]ProjectGroup, 0, len(groupMap))
	for _, g := range groupMap {
		// Sort projects within group by last modified.
		sort.Slice(g.Projects, func(i, j int) bool {
			return g.Projects[i].LastModified.After(g.Projects[j].LastModified)
		})
		groups = append(groups, *g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].LastModified.After(groups[j].LastModified)
	})

	return groups, nil
}
