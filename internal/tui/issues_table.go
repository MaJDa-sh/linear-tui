package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// Tree icons for expand/collapse indicators.
const (
	IconExpanded    = "▼"
	IconCollapsed   = "▶"
	IconChildPrefix = "└─"
)

// formatPriority formats a priority value into a display string with icon and label.
// Linear priority: 0 = No priority, 1 = Urgent, 2 = High, 3 = Normal, 4 = Low.
func formatPriority(priority int, theme Theme) (string, tcell.Color) {
	switch priority {
	case 1:
		return Icons.Priority + " Urgent", theme.StatusCanceled // Red for urgent
	case 2:
		return Icons.Priority + " High", theme.StatusInProgress // Yellow for high
	case 3:
		return Icons.Priority + " Normal", theme.Foreground // Default for normal
	case 4:
		return Icons.Priority + " Low", theme.SecondaryText // Gray for low
	default:
		return "-", theme.SecondaryText // No priority
	}
}

// getIssueFromRow returns the issue for a given table row (accounting for header).
// Returns nil if the row is invalid.
// This is a convenience wrapper that uses the current app's issueRows and idToIssue.
func (a *App) getIssueFromRow(row int) *linearapi.Issue {
	return getIssueFromRowModel(row, a.issueRows, a.idToIssue)
}

// getRowForIssue returns the table row for a given issue ID.
// Returns -1 if not found.
// This is a convenience wrapper that uses the current app's issueRows.
func (a *App) getRowForIssue(issueID string) int {
	return getRowForIssueModel(issueID, a.issueRows)
}

// getIssueFromRowModel returns the issue for a given table row using the provided model.
// Returns nil if the row is invalid.
func getIssueFromRowModel(row int, rows []IssueRow, idToIssue map[string]*linearapi.Issue) *linearapi.Issue {
	rowIndex := row - 1 // Account for header row
	if rowIndex < 0 || rowIndex >= len(rows) {
		return nil
	}
	issueID := rows[rowIndex].IssueID
	if issue, ok := idToIssue[issueID]; ok {
		return issue
	}
	return nil
}

// getRowForIssueModel returns the table row for a given issue ID using the provided model.
// Returns -1 if not found.
func getRowForIssueModel(issueID string, rows []IssueRow) int {
	for i, row := range rows {
		if row.IssueID == issueID {
			return i + 1 // +1 for header row
		}
	}
	return -1
}

// IssuesSection represents which issues section is active.
type IssuesSection int

const (
	IssuesSectionMy IssuesSection = iota
	IssuesSectionOther
)

// buildIssuesTable creates and configures an issues table widget with the given title.
// The table will use the provided getIssue and getRow functions for lookups.
func (a *App) buildIssuesTable(title string, section IssuesSection) *tview.Table {
	table := tview.NewTable()
	table.SetBorders(false). // Remove cell borders for cleaner look
					SetSelectable(true, false).
					SetBorder(true).
					SetTitle(title).
					SetTitleColor(a.theme.Foreground).
					SetBorderColor(a.theme.Border).
					SetBackgroundColor(a.theme.Background)

	table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(a.theme.SelectionText).
		Background(a.theme.SelectionBg).
		Bold(true))

	// Set column headers with better styling
	headerStyle := tcell.StyleDefault.
		Foreground(a.theme.HeaderText).
		Background(a.theme.HeaderBg).
		Bold(true)

	table.SetCell(0, 0, tview.NewTableCell(" ID").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 1, tview.NewTableCell("State").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 2, tview.NewTableCell("Priority").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 3, tview.NewTableCell("Assignee").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(2))
	table.SetCell(0, 4, tview.NewTableCell("Title").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(6))

	// Set fixed column widths
	table.SetFixed(1, 0)

	// Handle selection (Enter to open details or toggle expand)
	table.SetSelectedFunc(func(row, _ int) {
		issue := a.getIssueFromRowForSection(row, section)
		if issue == nil {
			return
		}

		// If issue has children, toggle expand/collapse
		if len(issue.Children) > 0 {
			a.toggleIssueExpanded(issue.ID)
			return
		}

		// Otherwise, focus on details
		a.onIssueSelected(*issue)
		a.focusedPane = FocusDetails
		a.updateFocus()
	})

	// Set up keyboard navigation with cross-section support
	a.setupIssuesTableNavigation(table, section)

	return table
}

// setupIssuesTableNavigation sets up keyboard navigation for an issues table with cross-section support.
func (a *App) setupIssuesTableNavigation(table *tview.Table, section IssuesSection) {
	// moveAndSelect moves the table cursor to the given row and triggers issue selection if applicable.
	moveAndSelect := func(row int, sec IssuesSection) {
		t := a.myIssuesTable
		if sec == IssuesSectionOther {
			t = a.otherIssuesTable
		}
		t.Select(row, 0)
		if issueRow := a.getIssueRowForSection(row, sec); issueRow != nil && !issueRow.IsStageHeader {
			if issue := a.getIssueFromRowForSection(row, sec); issue != nil {
				a.onIssueSelected(*issue)
			}
		}
		a.activeIssuesSection = sec
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, _ := table.GetSelection()
				if row < table.GetRowCount()-1 {
					moveAndSelect(row+1, section)
				} else if section == IssuesSectionMy && len(a.otherIssueRows) > 0 {
					// At bottom of My Issues — move to Other Issues.
					a.activeIssuesSection = IssuesSectionOther
					a.otherIssuesTable.Select(1, 0)
					if issueRow := a.getIssueRowForSection(1, IssuesSectionOther); issueRow != nil && !issueRow.IsStageHeader {
						if issue := a.getIssueFromRowForSection(1, IssuesSectionOther); issue != nil {
							a.onIssueSelected(*issue)
						}
					}
					a.updateFocus()
				}
				return nil
			case 'k':
				row, _ := table.GetSelection()
				if row > 1 {
					moveAndSelect(row-1, section)
				} else if section == IssuesSectionOther && len(a.myIssueRows) > 0 {
					// At top of Other Issues — move to My Issues.
					a.activeIssuesSection = IssuesSectionMy
					lastRow := len(a.myIssueRows)
					a.myIssuesTable.Select(lastRow, 0)
					if issueRow := a.getIssueRowForSection(lastRow, IssuesSectionMy); issueRow != nil && !issueRow.IsStageHeader {
						if issue := a.getIssueFromRowForSection(lastRow, IssuesSectionMy); issue != nil {
							a.onIssueSelected(*issue)
						}
					}
					a.updateFocus()
				}
				return nil
			case 'l':
				// Expand current parent issue (or ignore if on stage header).
				row, _ := table.GetSelection()
				if issueRow := a.getIssueRowForSection(row, section); issueRow != nil && issueRow.IsStageHeader {
					return nil
				}
				if issue := a.getIssueFromRowForSection(row, section); issue != nil {
					if len(issue.Children) > 0 && !a.expandedState[issue.ID] {
						a.toggleIssueExpanded(issue.ID)
						a.activeIssuesSection = section
					}
				}
				return nil
			case 'h':
				// Collapse current parent issue, or go to parent if on child.
				row, _ := table.GetSelection()
				if issueRow := a.getIssueRowForSection(row, section); issueRow != nil && issueRow.IsStageHeader {
					return nil
				}
				if issue := a.getIssueFromRowForSection(row, section); issue != nil {
					if len(issue.Children) > 0 && a.expandedState[issue.ID] {
						a.toggleIssueExpanded(issue.ID)
						a.activeIssuesSection = section
					} else if issue.Parent != nil {
						parentRow := a.getRowForIssueInSection(issue.Parent.ID, IssuesSectionMy)
						if parentRow > 0 {
							a.activeIssuesSection = IssuesSectionMy
							a.myIssuesTable.Select(parentRow, 0)
							if parent := a.getIssueFromRowForSection(parentRow, IssuesSectionMy); parent != nil {
								a.onIssueSelected(*parent)
							}
							a.updateFocus()
						} else {
							parentRow = a.getRowForIssueInSection(issue.Parent.ID, IssuesSectionOther)
							if parentRow > 0 {
								a.activeIssuesSection = IssuesSectionOther
								a.otherIssuesTable.Select(parentRow, 0)
								if parent := a.getIssueFromRowForSection(parentRow, IssuesSectionOther); parent != nil {
									a.onIssueSelected(*parent)
								}
								a.updateFocus()
							}
						}
					}
				}
				return nil
			case ' ':
				// Space toggles stage collapse or issue expand/collapse.
				row, _ := table.GetSelection()
				if issueRow := a.getIssueRowForSection(row, section); issueRow != nil {
					if issueRow.IsStageHeader {
						a.toggleStageCollapsed(issueRow.Stage)
						return nil
					}
				}
				if issue := a.getIssueFromRowForSection(row, section); issue != nil {
					if len(issue.Children) > 0 {
						a.toggleIssueExpanded(issue.ID)
						a.activeIssuesSection = section
					}
				}
				return nil
			}
		case tcell.KeyEnter:
			row, _ := table.GetSelection()

			// Handle stage header toggle.
			if issueRow := a.getIssueRowForSection(row, section); issueRow != nil && issueRow.IsStageHeader {
				a.toggleStageCollapsed(issueRow.Stage)
				return nil
			}

			issue := a.getIssueFromRowForSection(row, section)
			if issue == nil {
				return nil
			}

			// If issue has children, toggle expand/collapse.
			if len(issue.Children) > 0 {
				a.toggleIssueExpanded(issue.ID)
				a.activeIssuesSection = section
				return nil
			}

			// Otherwise, focus on details.
			a.onIssueSelected(*issue)
			a.focusedPane = FocusDetails
			a.updateFocus()
			return nil
		case tcell.KeyDown:
			row, _ := table.GetSelection()
			if row < table.GetRowCount()-1 {
				moveAndSelect(row+1, section)
			} else if section == IssuesSectionMy && len(a.otherIssueRows) > 0 {
				a.activeIssuesSection = IssuesSectionOther
				a.otherIssuesTable.Select(1, 0)
				if issueRow := a.getIssueRowForSection(1, IssuesSectionOther); issueRow != nil && !issueRow.IsStageHeader {
					if issue := a.getIssueFromRowForSection(1, IssuesSectionOther); issue != nil {
						a.onIssueSelected(*issue)
					}
				}
				a.updateFocus()
			}
			return nil
		case tcell.KeyUp:
			row, _ := table.GetSelection()
			if row > 1 {
				moveAndSelect(row-1, section)
			} else if section == IssuesSectionOther && len(a.myIssueRows) > 0 {
				a.activeIssuesSection = IssuesSectionMy
				lastRow := len(a.myIssueRows)
				a.myIssuesTable.Select(lastRow, 0)
				if issueRow := a.getIssueRowForSection(lastRow, IssuesSectionMy); issueRow != nil && !issueRow.IsStageHeader {
					if issue := a.getIssueFromRowForSection(lastRow, IssuesSectionMy); issue != nil {
						a.onIssueSelected(*issue)
					}
				}
				a.updateFocus()
			}
			return nil
		}
		return event
	})
}

// getIssueFromRowForSection returns the issue for a given table row in the specified section.
func (a *App) getIssueFromRowForSection(row int, section IssuesSection) *linearapi.Issue {
	var rows []IssueRow
	var idToIssue map[string]*linearapi.Issue
	switch section {
	case IssuesSectionMy:
		rows = a.myIssueRows
		idToIssue = a.myIDToIssue
	case IssuesSectionOther:
		rows = a.otherIssueRows
		idToIssue = a.otherIDToIssue
	}
	return getIssueFromRowModel(row, rows, idToIssue)
}

// getIssueRowForSection returns the raw IssueRow (including stage headers) for a given table row.
func (a *App) getIssueRowForSection(row int, section IssuesSection) *IssueRow {
	var rows []IssueRow
	switch section {
	case IssuesSectionMy:
		rows = a.myIssueRows
	case IssuesSectionOther:
		rows = a.otherIssueRows
	}
	rowIndex := row - 1 // Account for header row
	if rowIndex < 0 || rowIndex >= len(rows) {
		return nil
	}
	return &rows[rowIndex]
}

// getRowForIssueInSection returns the table row for a given issue ID in the specified section.
func (a *App) getRowForIssueInSection(issueID string, section IssuesSection) int {
	var rows []IssueRow
	switch section {
	case IssuesSectionMy:
		rows = a.myIssueRows
	case IssuesSectionOther:
		rows = a.otherIssueRows
	}
	return getRowForIssueModel(issueID, rows)
}

// renderIssuesTableModel renders a table with the given rows and issue lookup map.
func renderIssuesTableModel(table *tview.Table, rows []IssueRow, idToIssue map[string]*linearapi.Issue, selectedIssueID string, theme Theme) {
	table.Clear()

	// Set column headers with better styling
	headerStyle := tcell.StyleDefault.
		Foreground(theme.HeaderText).
		Background(theme.HeaderBg).
		Bold(true)

	table.SetCell(0, 0, tview.NewTableCell(" ID").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 1, tview.NewTableCell("State").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 2, tview.NewTableCell("Priority").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(1))
	table.SetCell(0, 3, tview.NewTableCell("Assignee").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(2))
	table.SetCell(0, 4, tview.NewTableCell("Title").
		SetStyle(headerStyle).
		SetAlign(tview.AlignLeft).
		SetSelectable(false).
		SetExpansion(6))

	// Add issue rows using the hierarchical structure
	for i, issueRow := range rows {
		row := i + 1

		// Render stage header rows differently.
		if issueRow.IsStageHeader {
			icon := IconCollapsed
			// We need to know if this stage is collapsed — check by looking ahead.
			// Since we only have the rows (not collapsedStages map here), we infer from
			// whether the next row is also a stage header or there are no issue rows following.
			// Simpler: store IsExpanded on stage header rows (we use IsExpanded for this).
			// Actually IsExpanded is not set for stage headers; use the absence of subsequent issue rows.
			// For display: if the next row exists and is NOT a stage header, we're expanded.
			nextIsIssue := i+1 < len(rows) && !rows[i+1].IsStageHeader
			if nextIsIssue || (i+1 >= len(rows)) {
				// Expanded if there are issue rows after (or it's the last header with no issues)
				// Check more carefully: if no issues follow before the next header, it's collapsed or empty.
				// Walk forward to see if any non-header row exists before next header.
				hasIssueRows := false
				for j := i + 1; j < len(rows); j++ {
					if rows[j].IsStageHeader {
						break
					}
					hasIssueRows = true
					break
				}
				if hasIssueRows {
					icon = IconExpanded
				}
			}

			stageText := fmt.Sprintf(" %s  %s  (%d)", icon, issueRow.Stage, issueRow.StageCount)
			stageStyle := tcell.StyleDefault.
				Foreground(theme.HeaderText).
				Background(theme.HeaderBg).
				Bold(true)

			table.SetCell(row, 0, tview.NewTableCell(stageText).
				SetStyle(stageStyle).
				SetSelectable(true).
				SetAlign(tview.AlignLeft))
			for col := 1; col <= 4; col++ {
				table.SetCell(row, col, tview.NewTableCell("").
					SetStyle(stageStyle).
					SetSelectable(false).
					SetAlign(tview.AlignLeft))
			}
			continue
		}

		issue, ok := idToIssue[issueRow.IssueID]
		if !ok || issue == nil {
			continue
		}

		// Build identifier with hierarchy indicator
		identifier := issue.Identifier
		identifierPrefix := " "

		if issueRow.Level > 0 {
			// Child issue - show indent prefix
			identifierPrefix = " " + IconChildPrefix + " "
		} else if issueRow.HasChildren {
			// Parent issue - show expand/collapse indicator
			if issueRow.IsExpanded {
				identifierPrefix = " " + IconExpanded + " "
			} else {
				identifierPrefix = " " + IconCollapsed + " "
			}
		}

		table.SetCell(row, 0, tview.NewTableCell(identifierPrefix+identifier).
			SetTextColor(theme.SecondaryText).
			SetAlign(tview.AlignLeft))

		// State with color based on state
		state := issue.State
		var stateColor tcell.Color
		var stateIcon string

		// Color code states
		lowerState := strings.ToLower(state)
		switch {
		case strings.Contains(lowerState, "done") || strings.Contains(lowerState, "complete"):
			stateColor = theme.StatusDone
			stateIcon = Icons.Done
		case strings.Contains(lowerState, "progress"):
			stateColor = theme.StatusInProgress
			stateIcon = Icons.InProgress
		case strings.Contains(lowerState, "cancel"):
			stateColor = theme.StatusCanceled
			stateIcon = Icons.Done
		default:
			stateColor = theme.StatusTodo
			stateIcon = Icons.Todo
		}

		if len(state) > 12 {
			state = state[:12]
		}

		table.SetCell(row, 1, tview.NewTableCell(stateIcon+" "+state).
			SetTextColor(stateColor).
			SetAlign(tview.AlignLeft))

		// Priority
		priorityText, priorityColor := formatPriority(issue.Priority, theme)
		table.SetCell(row, 2, tview.NewTableCell(priorityText).
			SetTextColor(priorityColor).
			SetAlign(tview.AlignLeft))

		// Assignee
		assignee := issue.Assignee
		assigneeColor := theme.Foreground
		if assignee == "" {
			assignee = "Unassigned"
			assigneeColor = theme.SecondaryText
		}
		if len(assignee) > 15 {
			assignee = assignee[:15]
		}

		table.SetCell(row, 3, tview.NewTableCell(assignee).
			SetTextColor(assigneeColor).
			SetAlign(tview.AlignLeft))

		// Title
		title := issue.Title
		table.SetCell(row, 4, tview.NewTableCell(title).
			SetTextColor(theme.Foreground).
			SetAlign(tview.AlignLeft))
	}

	// Select the specified issue or first non-header row
	if len(rows) > 0 {
		// Default: find first non-stage-header row
		selectedRow := 1
		for i, row := range rows {
			if !row.IsStageHeader {
				selectedRow = i + 1 // +1 because row 0 is the column header
				break
			}
		}
		if selectedIssueID != "" {
			// Find the row with matching issue ID
			for i, row := range rows {
				if row.IssueID == selectedIssueID {
					selectedRow = i + 1 // +1 because row 0 is header
					break
				}
			}
		}
		table.Select(selectedRow, 0)
	} else {
		// Show empty state message
		table.SetCell(1, 0, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 1, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 2, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 3, tview.NewTableCell("No issues").
			SetTextColor(theme.SecondaryText).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
		table.SetCell(1, 4, tview.NewTableCell("").SetSelectable(false))
	}
}

// renderIssueRow formats an issue for display in the table.
// This is a helper function that can be used for testing.
func renderIssueRow(issue linearapi.Issue) []string {
	identifier := issue.Identifier
	if len(identifier) > 10 {
		identifier = identifier[:10]
	}

	state := issue.State
	if len(state) > 10 {
		state = state[:10]
	}

	priorityText, _ := formatPriority(issue.Priority, LinearTheme)

	assignee := issue.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	if len(assignee) > 10 {
		assignee = assignee[:10]
	}

	return []string{identifier, state, priorityText, assignee, issue.Title}
}
