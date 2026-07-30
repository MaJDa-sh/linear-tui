package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// buildNotificationsTable creates and configures the notifications table widget.
func (a *App) buildNotificationsTable() *tview.Table {
	table := tview.NewTable()
	table.SetBorders(false).
		SetSelectable(true, false).
		SetBorder(true).
		SetTitle(" Inbox ").
		SetTitleColor(a.theme.Foreground).
		SetBorderColor(a.theme.Border).
		SetBackgroundColor(a.theme.Background)

	table.SetSelectedStyle(tcell.StyleDefault.
		Foreground(a.theme.SelectionText).
		Background(a.theme.SelectionBg).
		Bold(true))

	table.SetSelectionChangedFunc(func(row, _ int) {
		if n := a.getNotificationFromRow(row); n != nil {
			a.onNotificationSelected(n)
		}
	})

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				row, _ := table.GetSelection()
				if row < table.GetRowCount()-1 {
					table.Select(row+1, 0)
				}
				return nil
			case 'k':
				row, _ := table.GetSelection()
				if row > 1 {
					table.Select(row-1, 0)
				}
				return nil
			case 'o':
				if a.selectedNotification != nil && a.selectedNotification.IssueURL != "" {
					openURL(a.selectedNotification.IssueURL)
				}
				return nil
			}
		case tcell.KeyDown:
			row, _ := table.GetSelection()
			if row < table.GetRowCount()-1 {
				table.Select(row+1, 0)
			}
			return nil
		case tcell.KeyUp:
			row, _ := table.GetSelection()
			if row > 1 {
				table.Select(row-1, 0)
			}
			return nil
		case tcell.KeyEnter:
			if a.selectedNotification != nil && a.selectedNotification.IssueURL != "" {
				openURL(a.selectedNotification.IssueURL)
			}
			return nil
		}
		return event
	})

	return table
}

// onNotificationSelected updates state and the details view when a notification is selected.
func (a *App) onNotificationSelected(n *linearapi.Notification) {
	a.selectedNotification = n
	a.updateDetailsViewForNotification(n)
}

// getNotificationFromRow returns the notification for a given table row.
func (a *App) getNotificationFromRow(row int) *linearapi.Notification {
	idx := row - 1
	if idx < 0 || idx >= len(a.notifications) {
		return nil
	}
	return &a.notifications[idx]
}

// renderNotificationsTable populates the notifications table with data.
func (a *App) renderNotificationsTable() {
	table := a.notificationsTable
	table.Clear()

	headerStyle := tcell.StyleDefault.
		Foreground(a.theme.HeaderText).
		Background(a.theme.HeaderBg).
		Bold(true)

	table.SetCell(0, 0, tview.NewTableCell("").SetStyle(headerStyle).SetSelectable(false).SetExpansion(0))
	table.SetCell(0, 1, tview.NewTableCell("Type").SetStyle(headerStyle).SetSelectable(false).SetExpansion(2))
	table.SetCell(0, 2, tview.NewTableCell("Issue").SetStyle(headerStyle).SetSelectable(false).SetExpansion(1))
	table.SetCell(0, 3, tview.NewTableCell("Title").SetStyle(headerStyle).SetSelectable(false).SetExpansion(5))
	table.SetCell(0, 4, tview.NewTableCell("From").SetStyle(headerStyle).SetSelectable(false).SetExpansion(2))
	table.SetCell(0, 5, tview.NewTableCell("Time").SetStyle(headerStyle).SetSelectable(false).SetExpansion(1))
	table.SetFixed(1, 0)

	if len(a.notifications) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 1, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 2, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 3, tview.NewTableCell("No notifications").
			SetTextColor(a.theme.SecondaryText).
			SetAlign(tview.AlignCenter).
			SetSelectable(false))
		table.SetCell(1, 4, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(1, 5, tview.NewTableCell("").SetSelectable(false))
		return
	}

	for i, n := range a.notifications {
		row := i + 1

		readIcon := "●"
		readColor := a.theme.Accent
		if n.ReadAt != nil {
			readIcon = "○"
			readColor = a.theme.SecondaryText
		}
		table.SetCell(row, 0, tview.NewTableCell(readIcon).SetTextColor(readColor).SetAlign(tview.AlignCenter).SetSelectable(true))

		typeText := formatNotificationType(n.Type)
		typeColor := a.theme.Foreground
		if n.ReadAt != nil {
			typeColor = a.theme.SecondaryText
		}
		table.SetCell(row, 1, tview.NewTableCell(typeText).SetTextColor(typeColor).SetSelectable(true))

		issueID := n.IssueIdentifier
		if issueID == "" {
			issueID = "-"
		}
		table.SetCell(row, 2, tview.NewTableCell(issueID).SetTextColor(a.theme.SecondaryText).SetSelectable(true))

		title := n.IssueTitle
		if title == "" {
			title = "-"
		}
		titleColor := a.theme.Foreground
		if n.ReadAt != nil {
			titleColor = a.theme.SecondaryText
		}
		table.SetCell(row, 3, tview.NewTableCell(title).SetTextColor(titleColor).SetSelectable(true))

		actor := n.ActorName
		if actor == "" {
			actor = "-"
		}
		table.SetCell(row, 4, tview.NewTableCell(actor).SetTextColor(a.theme.SecondaryText).SetSelectable(true))

		table.SetCell(row, 5, tview.NewTableCell(formatCreatedAt(n.CreatedAt)).SetTextColor(a.theme.SecondaryText).SetSelectable(true))
	}

	table.Select(1, 0)
}

// formatNotificationType formats a notification type string for display.
func formatNotificationType(t string) string {
	switch t {
	case "issueAssigned":
		return "Assigned"
	case "issueUnassigned":
		return "Unassigned"
	case "issueCreated":
		return "Created"
	case "issueCommented":
		return "Commented"
	case "issueStatusChanged":
		return "Status changed"
	case "issuePriorityChanged":
		return "Priority changed"
	case "issueLabelsChanged":
		return "Labels changed"
	case "issueDueDateChanged":
		return "Due date changed"
	case "issueMentioned":
		return "Mentioned"
	case "issueSubscribed":
		return "Subscribed"
	default:
		return camelToTitle(t)
	}
}

// camelToTitle converts a camelCase string to words with a capitalized first letter.
func camelToTitle(s string) string {
	if s == "" {
		return s
	}
	result := make([]byte, 0, len(s)+4)
	for i := 0; i < len(s); i++ {
		b := s[i]
		if i == 0 && b >= 'a' && b <= 'z' {
			b -= 32
		} else if i > 0 && b >= 'A' && b <= 'Z' {
			result = append(result, ' ')
		}
		result = append(result, b)
	}
	return string(result)
}
