package tui

import (
	"sort"
	"strings"

	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// IssueRow represents a single row in the issues table with hierarchy info.
type IssueRow struct {
	IssueID       string // Reference to the issue
	Level         int    // Nesting level (0 = top-level, 1 = child, etc.)
	IsParent      bool   // True if this issue has children
	HasChildren   bool   // True if this issue has children (same as IsParent for now)
	IsExpanded    bool   // True if children are shown (only meaningful when HasChildren is true)
	IsStageHeader bool   // True if this row is a stage (workflow state) group header
	Stage         string // Stage name, only meaningful when IsStageHeader is true
	StageCount    int    // Total issue count in stage (including children)
}

// stageSortPriority returns a sort priority for workflow stages.
// Lower number = shown first (active stages come before completed ones).
func stageSortPriority(stage string) int {
	lower := strings.ToLower(stage)
	switch {
	case strings.Contains(lower, "in progress") || strings.Contains(lower, "inprogress"):
		return 0
	case strings.Contains(lower, "todo"):
		return 1
	case strings.Contains(lower, "backlog"):
		return 2
	case strings.Contains(lower, "triage"):
		return 3
	case strings.Contains(lower, "done") || strings.Contains(lower, "complete"):
		return 10
	case strings.Contains(lower, "cancel"):
		return 11
	default:
		return 5
	}
}

// BuildIssueRows constructs a flattened list of rows for table rendering.
// It builds a hierarchical view where parent issues can be expanded/collapsed.
// Returns the rows and a map for quick issue lookup by ID.
func BuildIssueRows(issues []linearapi.Issue, expanded map[string]bool) ([]IssueRow, map[string]*linearapi.Issue) {
	idToIssue := make(map[string]*linearapi.Issue, len(issues))
	for i := range issues {
		idToIssue[issues[i].ID] = &issues[i]
	}

	// Separate parent issues (no parent in our list) from children
	// An issue is a "top-level" issue if:
	// 1. It has no parent (issue.Parent == nil), OR
	// 2. Its parent is not in our fetched list (orphan sub-issue)
	var topLevel []*linearapi.Issue
	childrenByParent := make(map[string][]*linearapi.Issue)

	for i := range issues {
		issue := &issues[i]
		if issue.Parent == nil {
			// No parent - this is a top-level issue
			topLevel = append(topLevel, issue)
		} else if _, parentInList := idToIssue[issue.Parent.ID]; parentInList {
			// Parent is in our list - group under parent
			childrenByParent[issue.Parent.ID] = append(childrenByParent[issue.Parent.ID], issue)
		} else {
			// Orphan sub-issue (parent not in list) - treat as top-level with marker
			topLevel = append(topLevel, issue)
		}
	}

	// Build rows
	var rows []IssueRow

	for _, issue := range topLevel {
		// Check if this issue has children in our list
		children := childrenByParent[issue.ID]
		hasChildren := len(children) > 0 || len(issue.Children) > 0
		isExpanded := expanded[issue.ID]

		rows = append(rows, IssueRow{
			IssueID:     issue.ID,
			Level:       0,
			IsParent:    hasChildren,
			HasChildren: hasChildren,
			IsExpanded:  isExpanded,
		})

		// If expanded, add children
		if hasChildren && isExpanded {
			// Use children from our fetched list if available
			if len(children) > 0 {
				// Sort children by identifier for consistent ordering
				sort.Slice(children, func(i, j int) bool {
					return children[i].Identifier < children[j].Identifier
				})

				for _, child := range children {
					childHasChildren := len(child.Children) > 0
					childExpanded := expanded[child.ID]

					rows = append(rows, IssueRow{
						IssueID:     child.ID,
						Level:       1,
						IsParent:    childHasChildren,
						HasChildren: childHasChildren,
						IsExpanded:  childExpanded,
					})
				}
			}
		}
	}

	return rows, idToIssue
}

// BuildIssueRowsGrouped constructs a flattened list of rows with stage (workflow state) grouping.
// Each unique workflow state gets a collapsable header row. Issues within a collapsed stage are hidden.
func BuildIssueRowsGrouped(issues []linearapi.Issue, expanded map[string]bool, collapsedStages map[string]bool) ([]IssueRow, map[string]*linearapi.Issue) {
	idToIssue := make(map[string]*linearapi.Issue, len(issues))
	for i := range issues {
		idToIssue[issues[i].ID] = &issues[i]
	}

	// Separate top-level issues from children (same logic as BuildIssueRows).
	childrenByParent := make(map[string][]*linearapi.Issue)
	var topLevel []*linearapi.Issue
	for i := range issues {
		issue := &issues[i]
		if issue.Parent == nil {
			topLevel = append(topLevel, issue)
		} else if _, parentInList := idToIssue[issue.Parent.ID]; parentInList {
			childrenByParent[issue.Parent.ID] = append(childrenByParent[issue.Parent.ID], issue)
		} else {
			// Orphan sub-issue (parent not in list) - treat as top-level.
			topLevel = append(topLevel, issue)
		}
	}

	// Group top-level issues by workflow state.
	stageIssues := make(map[string][]*linearapi.Issue)
	var stageNames []string
	seenStages := make(map[string]bool)
	for _, issue := range topLevel {
		state := issue.State
		if state == "" {
			state = "No State"
		}
		if !seenStages[state] {
			stageNames = append(stageNames, state)
			seenStages[state] = true
		}
		stageIssues[state] = append(stageIssues[state], issue)
	}

	// Sort stages by priority (active stages first, completed last).
	sort.Slice(stageNames, func(i, j int) bool {
		pi := stageSortPriority(stageNames[i])
		pj := stageSortPriority(stageNames[j])
		if pi != pj {
			return pi < pj
		}
		return stageNames[i] < stageNames[j]
	})

	// Build rows with stage headers.
	var rows []IssueRow
	for _, stage := range stageNames {
		stageIssueList := stageIssues[stage]

		// Count total issues in stage (top-level + their children).
		totalCount := 0
		for _, issue := range stageIssueList {
			totalCount++
			totalCount += len(childrenByParent[issue.ID])
		}

		rows = append(rows, IssueRow{
			IsStageHeader: true,
			Stage:         stage,
			StageCount:    totalCount,
		})

		if collapsedStages[stage] {
			// Stage is collapsed — skip its issues.
			continue
		}

		for _, issue := range stageIssueList {
			children := childrenByParent[issue.ID]
			hasChildren := len(children) > 0 || len(issue.Children) > 0
			isExpanded := expanded[issue.ID]

			rows = append(rows, IssueRow{
				IssueID:     issue.ID,
				Level:       0,
				IsParent:    hasChildren,
				HasChildren: hasChildren,
				IsExpanded:  isExpanded,
			})

			if hasChildren && isExpanded {
				if len(children) > 0 {
					sort.Slice(children, func(i, j int) bool {
						return children[i].Identifier < children[j].Identifier
					})
					for _, child := range children {
						childHasChildren := len(child.Children) > 0
						childExpanded := expanded[child.ID]
						rows = append(rows, IssueRow{
							IssueID:     child.ID,
							Level:       1,
							IsParent:    childHasChildren,
							HasChildren: childHasChildren,
							IsExpanded:  childExpanded,
						})
					}
				}
			}
		}
	}

	return rows, idToIssue
}

// ToggleExpanded toggles the expanded state for an issue.
// Returns the new expanded state.
func ToggleExpanded(expanded map[string]bool, issueID string) bool {
	newState := !expanded[issueID]
	expanded[issueID] = newState
	return newState
}

// CollapseAll sets all issues to collapsed state.
func CollapseAll(expanded map[string]bool) {
	for k := range expanded {
		delete(expanded, k)
	}
}

// ExpandAll expands all parent issues.
func ExpandAll(expanded map[string]bool, issues []linearapi.Issue) {
	for _, issue := range issues {
		if len(issue.Children) > 0 || issue.Parent == nil {
			// Expand issues that have children
			expanded[issue.ID] = true
		}
	}
}
