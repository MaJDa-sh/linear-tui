package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/roeyazroel/linear-tui/internal/linearapi"
)

// markdownRenderer is a shared glamour renderer for markdown content.
var markdownRenderer *glamour.TermRenderer

// initMarkdownRenderer initializes the glamour markdown renderer.
func initMarkdownRenderer() {
	var err error
	markdownRenderer, err = glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Fallback: create a basic renderer if custom style fails
		markdownRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(80),
		)
	}
}

// renderMarkdown renders markdown content using glamour.
// Falls back to plain text if rendering fails.
func renderMarkdown(content string) string {
	if markdownRenderer == nil {
		initMarkdownRenderer()
	}

	rendered, err := markdownRenderer.Render(content)
	if err != nil {
		// Fallback to plain text on error
		return content
	}

	// Trim extra whitespace that glamour may add
	return strings.TrimSpace(rendered)
}

// buildDetailsView creates and configures the details view with separate description and comments sections.
func (a *App) buildDetailsView() *tview.Flex {
	// Create description/metadata view (top section, scrollable)
	a.detailsDescriptionView = tview.NewTextView()
	a.detailsDescriptionView.SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle(" Details ").
		SetTitleColor(a.theme.Foreground).
		SetBorderColor(a.theme.Border).
		SetBackgroundColor(tcell.ColorDefault)
	padding := a.density.DetailsPadding
	a.detailsDescriptionView.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Create comments view (bottom section, scrollable, fixed height)
	a.detailsCommentsView = tview.NewTextView()
	a.detailsCommentsView.SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBorder(true).
		SetTitle(" Comments ").
		SetTitleColor(a.theme.Foreground).
		SetBorderColor(a.theme.Border).
		SetBackgroundColor(tcell.ColorDefault)
	a.detailsCommentsView.SetBorderPadding(padding.Top, padding.Bottom, padding.Left, padding.Right)

	// Create flex layout; comments are added conditionally after issue selection.
	detailsFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	a.detailsView = detailsFlex
	a.setDetailsCommentsVisibility(false)

	return a.detailsView
}

// setDetailsCommentsVisibility rebuilds the details layout to show or hide comments.
func (a *App) setDetailsCommentsVisibility(showComments bool) {
	if a.detailsView == nil || a.detailsDescriptionView == nil || a.detailsCommentsView == nil {
		return
	}
	if a.detailsCommentsVisible == showComments && a.detailsView.GetItemCount() > 0 {
		return
	}

	a.detailsView.Clear().
		AddItem(a.detailsDescriptionView, 0, 3, true)
	if showComments {
		a.detailsView.AddItem(a.detailsCommentsView, 0, 2, false)
	}

	a.detailsCommentsVisible = showComments
	if !showComments {
		a.focusedDetailsView = false
	}
}

// updateDetailsView updates the details view with the selected issue.
func (a *App) updateDetailsView() {
	a.issuesMu.RLock()
	selectedIssue := a.selectedIssue
	a.issuesMu.RUnlock()
	hasComments := selectedIssue != nil && len(selectedIssue.Comments) > 0
	a.setDetailsCommentsVisibility(hasComments)
	if selectedIssue == nil {
		a.detailsDescriptionView.SetText(fmt.Sprintf("%sNo issue selected. Select an issue from the list to view details.[-]", a.themeTags.SecondaryText))
		a.detailsCommentsView.SetText("")
		if a.focusedPane == FocusDetails && !a.detailsCommentsVisible {
			a.updateFocus()
		}
		return
	}

	issue := selectedIssue

	// Helper to colorize keys
	keyColor := a.themeTags.SecondaryText
	valColor := a.themeTags.Foreground
	accentColor := a.themeTags.Accent
	dividerColor := a.themeTags.Border
	sectionGap := a.density.DetailsSectionGap

	// ===== Update Description/Metadata View =====
	var headerLines []string

	// Issue header info with styling
	headerLines = append(headerLines, fmt.Sprintf("%s%s[-]", accentColor, issue.Identifier))
	headerLines = append(headerLines, fmt.Sprintf("[b]%s%s[-]", valColor, issue.Title))
	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}

	// Metadata grid simulation
	headerLines = append(headerLines, fmt.Sprintf("%sState:[-]      %s%s[-]", keyColor, valColor, issue.State))

	assignee := "Unassigned"
	if issue.Assignee != "" {
		assignee = issue.Assignee
	}
	headerLines = append(headerLines, fmt.Sprintf("%sAssignee:[-]   %s%s[-]", keyColor, valColor, assignee))

	headerLines = append(headerLines, fmt.Sprintf("%sPriority:[-]   %s%d[-]", keyColor, valColor, issue.Priority))

	// Dates
	if !issue.CreatedAt.IsZero() {
		headerLines = append(headerLines, fmt.Sprintf("%sCreated:[-]    %s%s[-]", keyColor, valColor, issue.CreatedAt.Format("Jan 2, 2006")))
	}
	if issue.DueDate != nil {
		dueDateColor := valColor
		if issue.DueDate.Before(time.Now()) {
			dueDateColor = a.themeTags.Error
		}
		headerLines = append(headerLines, fmt.Sprintf("%sDue:[-]        %s%s[-]", keyColor, dueDateColor, issue.DueDate.Format("Jan 2, 2006")))
	}

	// Labels
	labelsText := "No labels"
	if len(issue.Labels) > 0 {
		labelNames := make([]string, len(issue.Labels))
		for i, lbl := range issue.Labels {
			labelNames[i] = lbl.Name
		}
		labelsText = strings.Join(labelNames, ", ")
	}
	headerLines = append(headerLines, fmt.Sprintf("%sLabels:[-]     %s%s[-]", keyColor, valColor, labelsText))

	// Parent issue (if this is a sub-issue)
	if issue.Parent != nil {
		parentText := fmt.Sprintf("%s - %s", issue.Parent.Identifier, issue.Parent.Title)
		headerLines = append(headerLines, fmt.Sprintf("%sParent:[-]     %s%s[-]", keyColor, accentColor, parentText))
	}

	// Sub-issues (if this is a parent issue)
	if len(issue.Children) > 0 {
		for i := 0; i < sectionGap; i++ {
			headerLines = append(headerLines, "")
		}
		headerLines = append(headerLines, fmt.Sprintf("%sSub-issues:[-] %s%d items[-]", keyColor, valColor, len(issue.Children)))
		for _, child := range issue.Children {
			// Show child identifier, state, and title
			childLine := fmt.Sprintf("  %s└─[-] %s%s[-] %s[%s][-] %s%s[-]",
				keyColor,
				accentColor, child.Identifier,
				keyColor, child.State,
				valColor, child.Title)
			headerLines = append(headerLines, childLine)
		}
	}

	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}
	headerLines = append(headerLines, fmt.Sprintf("%s────────────────────────────────────────[-]", dividerColor))
	for i := 0; i < sectionGap; i++ {
		headerLines = append(headerLines, "")
	}

	// Set header first, then append description via ANSIWriter
	a.detailsDescriptionView.Clear()
	a.detailsDescriptionView.SetText(strings.Join(headerLines, "\n"))
	writer := tview.ANSIWriter(a.detailsDescriptionView)

	// Description
	if issue.Description != "" {
		_, _ = fmt.Fprintf(writer, "%sDescription:[-]\n\n", keyColor)

		// Render description as markdown and write through ANSIWriter
		// ANSIWriter translates ANSI escape codes to tview color tags
		renderedDesc := renderMarkdown(issue.Description)
		_, _ = fmt.Fprint(writer, renderedDesc)
	} else {
		_, _ = fmt.Fprintf(writer, "%sNo description available[-]", keyColor)
	}

	a.detailsDescriptionView.ScrollToBeginning()

	// ===== Update Comments View =====
	a.detailsCommentsView.Clear()
	commentsWriter := tview.ANSIWriter(a.detailsCommentsView)

	if len(issue.Comments) > 0 {
		_, _ = fmt.Fprintf(commentsWriter, "%sComments:[-] (%d)\n\n", keyColor, len(issue.Comments))

		for i, comment := range issue.Comments {
			// Comment header: author and timestamp
			authorDisplay := comment.Author.DisplayName
			if authorDisplay == "" {
				authorDisplay = comment.Author.Name
			}
			if comment.Author.IsMe {
				authorDisplay = fmt.Sprintf("%s (me)", authorDisplay)
			}

			// Format timestamp
			timeStr := comment.CreatedAt.Format("Jan 2, 2006 3:04 PM")
			if !comment.UpdatedAt.Equal(comment.CreatedAt) {
				timeStr += " (edited)"
			}

			_, _ = fmt.Fprintf(commentsWriter, "%s%s[-] %s%s[-]\n", accentColor, authorDisplay, keyColor, timeStr)
			_, _ = fmt.Fprint(commentsWriter, "\n")

			// Render comment body as markdown
			renderedComment := renderMarkdown(comment.Body)
			_, _ = fmt.Fprint(commentsWriter, renderedComment)

			// Add separator between comments (but not after the last one)
			if i < len(issue.Comments)-1 {
				_, _ = fmt.Fprint(commentsWriter, "\n\n")
				_, _ = fmt.Fprintf(commentsWriter, "%s────────────────────────────────────────[-]\n\n", dividerColor)
			}
		}
	} else {
		// Empty state for comments
		_, _ = fmt.Fprintf(commentsWriter, "%sNo comments yet.[-]", keyColor)
	}

	a.detailsCommentsView.ScrollToBeginning()
	if a.focusedPane == FocusDetails && !a.detailsCommentsVisible {
		a.updateFocus()
	}

	// Populate plain text lines for cursor mode.
	a.detailsTextLines = buildDetailsTextLines(issue)
	a.detailsCommentsLines = buildCommentsTextLines(issue)
	// Reset cursor mode state when issue changes.
	if a.detailsCursorMode {
		a.detailsCursorMode = false
		a.detailsVisualMode = false
		a.detailsVisualStart = -1
		a.detailsCursorLine = 0
		a.detailsCursorOnComments = false
	}
}

// updateDetailsViewForNotification renders a notification's details in the details pane.
func (a *App) updateDetailsViewForNotification(n *linearapi.Notification) {
	a.setDetailsCommentsVisibility(false)

	keyColor := a.themeTags.SecondaryText
	valColor := a.themeTags.Foreground
	accentColor := a.themeTags.Accent
	dividerColor := a.themeTags.Border
	sectionGap := a.density.DetailsSectionGap

	var lines []string

	typeLabel := formatNotificationType(n.Type)
	readStatus := "Unread"
	if n.ReadAt != nil {
		readStatus = "Read"
	}

	lines = append(lines, fmt.Sprintf("%s%s[-]  %s(%s)[-]", accentColor, typeLabel, keyColor, readStatus))
	for i := 0; i < sectionGap; i++ {
		lines = append(lines, "")
	}

	if n.IssueIdentifier != "" {
		lines = append(lines, fmt.Sprintf("%sIssue:[-]    %s%s[-]", keyColor, accentColor, n.IssueIdentifier))
	}
	if n.IssueTitle != "" {
		lines = append(lines, fmt.Sprintf("%sTitle:[-]    %s%s[-]", keyColor, valColor, n.IssueTitle))
	}
	if n.IssueState != "" {
		lines = append(lines, fmt.Sprintf("%sState:[-]    %s%s[-]", keyColor, valColor, n.IssueState))
	}
	if n.IssueAssignee != "" {
		lines = append(lines, fmt.Sprintf("%sAssignee:[-] %s%s[-]", keyColor, valColor, n.IssueAssignee))
	}
	if n.ActorName != "" {
		lines = append(lines, fmt.Sprintf("%sFrom:[-]     %s%s[-]", keyColor, valColor, n.ActorName))
	}
	if !n.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("%sTime:[-]     %s%s[-]", keyColor, valColor, n.CreatedAt.Format("Jan 2, 2006 3:04 PM")))
	}

	for i := 0; i < sectionGap; i++ {
		lines = append(lines, "")
	}
	lines = append(lines, fmt.Sprintf("%s────────────────────────────────────────[-]", dividerColor))
	for i := 0; i < sectionGap; i++ {
		lines = append(lines, "")
	}

	if n.IssueURL != "" {
		lines = append(lines, fmt.Sprintf("%sPress %so[-]%s or %sEnter[-]%s to open in browser.[-]", keyColor, accentColor, keyColor, accentColor, keyColor))
	}

	a.detailsDescriptionView.Clear()
	a.detailsDescriptionView.SetText(strings.Join(lines, "\n"))
	a.detailsDescriptionView.ScrollToBeginning()
}

// buildDetailsTextLines builds a slice of plain-text lines from an issue for cursor mode.
func buildDetailsTextLines(issue *linearapi.Issue) []string {
	if issue == nil {
		return nil
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("[%s] %s", issue.Identifier, issue.Title))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("State:    %s", issue.State))
	assignee := issue.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	lines = append(lines, fmt.Sprintf("Assignee: %s", assignee))
	lines = append(lines, fmt.Sprintf("Priority: %d", issue.Priority))
	if len(issue.Labels) > 0 {
		labelNames := make([]string, len(issue.Labels))
		for i, lbl := range issue.Labels {
			labelNames[i] = lbl.Name
		}
		lines = append(lines, fmt.Sprintf("Labels:   %s", strings.Join(labelNames, ", ")))
	}
	if issue.Parent != nil {
		lines = append(lines, fmt.Sprintf("Parent:   %s - %s", issue.Parent.Identifier, issue.Parent.Title))
	}
	lines = append(lines, "")
	lines = append(lines, strings.Repeat("─", 40))
	lines = append(lines, "")
	if issue.Description != "" {
		for _, line := range strings.Split(issue.Description, "\n") {
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, "(no description)")
	}
	return lines
}

// buildCommentsTextLines builds plain-text lines from the issue comments for cursor mode.
func buildCommentsTextLines(issue *linearapi.Issue) []string {
	if issue == nil || len(issue.Comments) == 0 {
		return []string{"(no comments)"}
	}
	var lines []string
	for i, comment := range issue.Comments {
		author := comment.Author.DisplayName
		if author == "" {
			author = comment.Author.Name
		}
		if comment.Author.IsMe {
			author += " (me)"
		}
		timeStr := comment.CreatedAt.Format("Jan 2, 2006 3:04 PM")
		if !comment.UpdatedAt.Equal(comment.CreatedAt) {
			timeStr += " (edited)"
		}
		lines = append(lines, fmt.Sprintf("%s  —  %s", author, timeStr))
		lines = append(lines, "")
		for _, bodyLine := range strings.Split(comment.Body, "\n") {
			lines = append(lines, bodyLine)
		}
		if i < len(issue.Comments)-1 {
			lines = append(lines, "")
			lines = append(lines, strings.Repeat("─", 40))
			lines = append(lines, "")
		}
	}
	return lines
}

// activeCursorLines returns the text lines for the currently active cursor mode pane.
func (a *App) activeCursorLines() []string {
	if a.detailsCursorOnComments {
		return a.detailsCommentsLines
	}
	return a.detailsTextLines
}

// activeCursorView returns the TextView for the currently active cursor mode pane.
func (a *App) activeCursorView() *tview.TextView {
	if a.detailsCursorOnComments {
		return a.detailsCommentsView
	}
	return a.detailsDescriptionView
}

// renderDetailsCursorMode re-renders the active details pane in cursor/visual mode
// with the cursor line and selection highlighted.
func (a *App) renderDetailsCursorMode() {
	view := a.activeCursorView()
	if view == nil {
		return
	}

	lines := a.activeCursorLines()
	if len(lines) == 0 {
		return
	}

	// Clamp cursor line.
	if a.detailsCursorLine >= len(lines) {
		a.detailsCursorLine = len(lines) - 1
	}
	if a.detailsCursorLine < 0 {
		a.detailsCursorLine = 0
	}

	// Determine selection range.
	selStart, selEnd := -1, -1
	if a.detailsVisualMode && a.detailsVisualStart >= 0 {
		selStart = a.detailsVisualStart
		selEnd = a.detailsCursorLine
		if selStart > selEnd {
			selStart, selEnd = selEnd, selStart
		}
	}

	// Build colored content.
	var sb strings.Builder
	for i, line := range lines {
		// Escape tview color tags in the plain text.
		escaped := strings.ReplaceAll(line, "[", "[[")

		if i == a.detailsCursorLine && !(selStart <= i && i <= selEnd) {
			// Cursor line (not in selection) — reverse video.
			sb.WriteString("[::r]")
			sb.WriteString(escaped)
			sb.WriteString("[-:-:-]\n")
		} else if selStart >= 0 && selStart <= i && i <= selEnd {
			// Selected lines — yellow bold.
			if i == a.detailsCursorLine {
				sb.WriteString("[black:yellow:b]")
			} else {
				sb.WriteString("[black:yellow:]")
			}
			sb.WriteString(escaped)
			sb.WriteString("[-:-:-]\n")
		} else {
			sb.WriteString(escaped)
			sb.WriteString("\n")
		}
	}

	// Update title to show cursor mode indicator.
	baseTitle := "Details"
	if a.detailsCursorOnComments {
		baseTitle = "Comments"
	}
	modeLabel := fmt.Sprintf(" ▶ %s [CURSOR] ", baseTitle)
	if a.detailsVisualMode {
		modeLabel = fmt.Sprintf(" ▶ %s [VISUAL] ", baseTitle)
	}
	view.SetTitle(modeLabel)
	view.SetTitleColor(a.theme.Accent)

	view.SetText(sb.String())
	// Scroll to keep cursor visible.
	view.ScrollTo(a.detailsCursorLine, 0)

	a.updateStatusBar()
}

// copyDetailsSelection copies the selected lines (or current line) to the system clipboard.
func (a *App) copyDetailsSelection() {
	lines := a.activeCursorLines()
	if len(lines) == 0 {
		return
	}

	var text string
	if a.detailsVisualMode && a.detailsVisualStart >= 0 {
		start := a.detailsVisualStart
		end := a.detailsCursorLine
		if start > end {
			start, end = end, start
		}
		if end >= len(lines) {
			end = len(lines) - 1
		}
		text = strings.Join(lines[start:end+1], "\n")
	} else {
		if a.detailsCursorLine < len(lines) {
			text = lines[a.detailsCursorLine]
		}
	}

	if err := copyToClipboard(text); err != nil {
		a.updateStatusBarWithError(err)
		return
	}

	// Exit visual mode after copy, keep cursor mode.
	a.detailsVisualMode = false
	a.detailsVisualStart = -1
	a.renderDetailsCursorMode()
	a.statusBar.SetText(fmt.Sprintf("%sCopied %d line(s) to clipboard[-]", a.themeTags.Accent, strings.Count(text, "\n")+1))
}

// openInEditor opens the content of the active details pane in $EDITOR.
// onComments selects the comments pane; otherwise the description pane is used.
func (a *App) openInEditor(onComments bool) {
	var lines []string
	var suffix string
	if onComments {
		lines = a.detailsCommentsLines
		suffix = "-comments.md"
	} else {
		lines = a.detailsTextLines
		suffix = "-description.md"
	}

	if len(lines) == 0 {
		return
	}

	content := strings.Join(lines, "\n")

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}

	// Write to a temp file.
	tmpFile, err := os.CreateTemp("", "linear-tui-*"+suffix)
	if err != nil {
		a.updateStatusBarWithError(fmt.Errorf("editor: cannot create temp file: %w", err))
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		a.updateStatusBarWithError(fmt.Errorf("editor: cannot write temp file: %w", err))
		return
	}
	tmpFile.Close()

	// Suspend tview and hand the terminal to the editor.
	a.app.Suspend(func() {
		cmd := exec.Command(editor, tmpPath) //nolint:gosec
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	})
}


