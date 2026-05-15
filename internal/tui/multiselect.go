package tui

import (
	"context"

	"github.com/roeyazroel/linear-tui/internal/linearapi"
	"github.com/roeyazroel/linear-tui/internal/logger"
)

// enterMultiSelect starts multi-select mode with the current row as anchor.
func (a *App) enterMultiSelect() {
	table := a.activeIssuesTable()
	if table == nil {
		return
	}
	row, _ := table.GetSelection()
	issue := a.getIssueFromRowForSection(row, a.activeIssuesSection)
	if issue == nil {
		return
	}
	a.multiSelectActive = true
	a.multiSelectSection = a.activeIssuesSection
	a.multiSelectAnchor = row
	a.multiSelectIDs = map[string]bool{issue.ID: true}
	a.rerenderTableForSection(a.multiSelectSection, row)
	a.updateStatusBar()
}

// exitMultiSelect exits multi-select mode and clears the selection.
func (a *App) exitMultiSelect() {
	section := a.multiSelectSection
	a.multiSelectActive = false
	a.multiSelectIDs = make(map[string]bool)
	// Re-render to clear highlights — preserve cursor position.
	table := a.myIssuesTable
	if section == IssuesSectionOther {
		table = a.otherIssuesTable
	}
	if table != nil {
		row, _ := table.GetSelection()
		a.rerenderTableForSection(section, row)
	}
	a.updateStatusBar()
}

// updateMultiSelectRange updates multiSelectIDs to cover all issue rows between
// the anchor and currentRow (inclusive), skipping stage headers.
func (a *App) updateMultiSelectRange(currentRow int) {
	rows := a.activeIssueRows()
	var idToIssue map[string]*linearapi.Issue
	if a.multiSelectSection == IssuesSectionMy {
		idToIssue = a.myIDToIssue
	} else {
		idToIssue = a.otherIDToIssue
	}

	anchor := a.multiSelectAnchor
	start, end := anchor, currentRow
	if start > end {
		start, end = end, start
	}

	a.multiSelectIDs = make(map[string]bool)
	for tableRow := start; tableRow <= end; tableRow++ {
		idx := tableRow - 1
		if idx < 0 || idx >= len(rows) {
			continue
		}
		r := rows[idx]
		if r.IsStageHeader {
			continue
		}
		if _, ok := idToIssue[r.IssueID]; ok {
			a.multiSelectIDs[r.IssueID] = true
		}
	}
}

// multiSelectIDsForSection returns the multi-select set for a section, or nil
// if multi-select is not active for that section.
func (a *App) multiSelectIDsForSection(section IssuesSection) map[string]bool {
	if a.multiSelectActive && a.multiSelectSection == section {
		return a.multiSelectIDs
	}
	return nil
}

// rerenderTableForSection re-renders the given section's table, placing the
// cursor on the issue at cursorTableRow (1-indexed).
func (a *App) rerenderTableForSection(section IssuesSection, cursorTableRow int) {
	var table = a.myIssuesTable
	var rows = a.myIssueRows
	var idToIssue = a.myIDToIssue
	if section == IssuesSectionOther {
		table = a.otherIssuesTable
		rows = a.otherIssueRows
		idToIssue = a.otherIDToIssue
	}
	if table == nil {
		return
	}

	selectedID := ""
	idx := cursorTableRow - 1
	if idx >= 0 && idx < len(rows) && !rows[idx].IsStageHeader {
		selectedID = rows[idx].IssueID
	}

	renderIssuesTableModel(table, rows, idToIssue, selectedID, a.theme, a.multiSelectIDsForSection(section))
}

// handleMultiSelectAction executes the given action on all currently selected issues.
func (a *App) handleMultiSelectAction(action rune) {
	issueIDs := make([]string, 0, len(a.multiSelectIDs))
	for id := range a.multiSelectIDs {
		issueIDs = append(issueIDs, id)
	}
	if len(issueIDs) == 0 {
		return
	}

	switch action {
	case 's':
		a.ShowStatusPicker(func(stateID string) {
			go func() {
				ctx := context.Background()
				for _, id := range issueIDs {
					_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
						ID:      id,
						StateID: &stateID,
					})
					if err != nil {
						a.QueueUpdateDraw(func() { a.updateStatusBarWithError(err) })
						return
					}
				}
				logger.Info("tui.multiselect: changed status for %d issues", len(issueIDs))
				a.QueueUpdateDraw(func() {
					a.exitMultiSelect()
					go a.refreshIssues()
				})
			}()
		})

	case 'm':
		user := a.GetCurrentUser()
		if user == nil {
			return
		}
		userID := user.ID
		go func() {
			ctx := context.Background()
			for _, id := range issueIDs {
				_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
					ID:         id,
					AssigneeID: &userID,
				})
				if err != nil {
					a.QueueUpdateDraw(func() { a.updateStatusBarWithError(err) })
					return
				}
			}
			logger.Info("tui.multiselect: assigned %d issues to me", len(issueIDs))
			a.QueueUpdateDraw(func() {
				a.exitMultiSelect()
				go a.refreshIssues()
			})
		}()

	case 'u':
		emptyAssignee := ""
		go func() {
			ctx := context.Background()
			for _, id := range issueIDs {
				_, err := a.GetAPI().UpdateIssue(ctx, linearapi.UpdateIssueInput{
					ID:         id,
					AssigneeID: &emptyAssignee,
				})
				if err != nil {
					a.QueueUpdateDraw(func() { a.updateStatusBarWithError(err) })
					return
				}
			}
			logger.Info("tui.multiselect: unassigned %d issues", len(issueIDs))
			a.QueueUpdateDraw(func() {
				a.exitMultiSelect()
				go a.refreshIssues()
			})
		}()
	}
}
