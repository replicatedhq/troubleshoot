package preflight

import (
	"fmt"
	"time"

	"github.com/mitchellh/go-wordwrap"
	"github.com/pkg/errors"
	ui "github.com/replicatedhq/termui/v3"
	"github.com/replicatedhq/termui/v3/widgets"
	"github.com/replicatedhq/troubleshoot/internal/util"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

var (
	selectedResult = 0
	table          = widgets.NewTable()
	isShowingSaved = false
)

func showInteractiveResults(preflightName string, outputPath string, analyzeResults []*analyzerunner.AnalyzeResult) error {
	if err := ui.Init(); err != nil {
		return errors.Wrap(err, "failed to create terminal ui")
	}
	defer ui.Close()

	drawUI(preflightName, analyzeResults)

	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "<C-c>":
				return nil
			case "q":
				if isShowingSaved == true {
					isShowingSaved = false
					ui.Clear()
					drawUI(preflightName, analyzeResults)
				} else {
					return nil
				}
			case "s":
				filename, err := outputToFile(preflightName, outputPath, analyzeResults)
				if err != nil {
					// show
				} else {
					showSaved(filename)
					go func() {
						time.Sleep(time.Second * 5)
						isShowingSaved = false
						ui.Clear()
						drawUI(preflightName, analyzeResults)
					}()
				}
			case "<Resize>":
				ui.Clear()
				drawUI(preflightName, analyzeResults)
			case "<Down>":
				if selectedResult < len(analyzeResults)-1 {
					selectedResult++
				} else {
					selectedResult = 0
					table.SelectedRow = 0
				}
				table.ScrollDown()
				ui.Clear()
				drawUI(preflightName, analyzeResults)
			case "<Up>":
				if selectedResult > 0 {
					selectedResult--
				} else {
					selectedResult = len(analyzeResults) - 1
					table.SelectedRow = len(analyzeResults)
				}
				table.ScrollUp()
				ui.Clear()
				drawUI(preflightName, analyzeResults)
			}
		}
	}
}

func drawUI(preflightName string, analyzeResults []*analyzerunner.AnalyzeResult) {
	drawGrid(analyzeResults)
	drawHeader(preflightName)
	drawFooter()
}

func drawGrid(analyzeResults []*analyzerunner.AnalyzeResult) {
	drawPreflightTable(analyzeResults)
	drawDetails(analyzeResults[selectedResult])
}

func drawHeader(preflightName string) {
	termWidth, _ := ui.TerminalDimensions()

	title := widgets.NewParagraph()
	title.Text = fmt.Sprintf("%s Preflight Checks", util.AppName(preflightName))
	title.TextStyle.Fg = ui.ColorWhite
	title.TextStyle.Bg = ui.ColorClear
	title.TextStyle.Modifier = ui.ModifierBold
	title.Border = false

	left := termWidth/2 - 2*len(title.Text)/3
	right := termWidth/2 + (termWidth/2 - left)

	title.SetRect(left, 0, right, 1)
	ui.Render(title)
}

func drawFooter() {
	termWidth, termHeight := ui.TerminalDimensions()

	instructions := widgets.NewParagraph()
	instructions.Text = "[q] quit    [s] save    [↑][↓] scroll"
	instructions.Border = false

	left := 0
	right := termWidth
	top := termHeight - 1
	bottom := termHeight

	instructions.SetRect(left, top, right, bottom)
	ui.Render(instructions)
}

func drawPreflightTable(analyzeResults []*analyzerunner.AnalyzeResult) {
	termWidth, termHeight := ui.TerminalDimensions()

	table.SetRect(0, 3, termWidth/2, termHeight-4)
	table.FillRow = true
	table.Border = true
	table.Rows = [][]string{}
	table.ColumnWidths = []int{termWidth}

	for i, analyzeResult := range analyzeResults {
		title := analyzeResult.Title
		if analyzeResult.Strict {
			title = title + fmt.Sprintf(" (Strict: %t)", analyzeResult.Strict)
		}
		if analyzeResult.IsPass {
			title = fmt.Sprintf("✔  %s", title)
		} else if analyzeResult.IsWarn {
			title = fmt.Sprintf("⚠️  %s", title)
		} else if analyzeResult.IsFail {
			title = fmt.Sprintf("✘  %s", title)
		}
		table.Rows = append(table.Rows, []string{
			title,
		})

		if analyzeResult.IsPass {
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorGreen, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorGreen, ui.ColorClear)
			}
		} else if analyzeResult.IsWarn {
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorYellow, ui.ColorClear)
			}
		} else if analyzeResult.IsFail {
			if i == selectedResult {
				table.RowStyles[i] = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierReverse)
			} else {
				table.RowStyles[i] = ui.NewStyle(ui.ColorRed, ui.ColorClear)
			}
		}
	}

	ui.Render(table)
}

func drawDetails(analysisResult *analyzerunner.AnalyzeResult) {
	termWidth, _ := ui.TerminalDimensions()

	currentTop := 4
	title := widgets.NewParagraph()
	// WrapText is set to false to prevent internal wordwrap which is not accounting for the padding
	title.WrapText = false
	// For long title that lead to wrapping text, the terminal width is divided by 2 and deducted by MESSAGE_TEXT_PADDING to account for the padding
	title.Text = wordwrap.WrapString(analysisResult.Title, uint(termWidth/2-constants.MESSAGE_TEXT_PADDING))
	title.Border = false
	if analysisResult.IsPass {
		title.TextStyle = ui.NewStyle(ui.ColorGreen, ui.ColorClear, ui.ModifierBold)
	} else if analysisResult.IsWarn {
		title.TextStyle = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
	} else if analysisResult.IsFail {
		title.TextStyle = ui.NewStyle(ui.ColorRed, ui.ColorClear, ui.ModifierBold)
	}
	height := util.EstimateNumberOfLines(title.Text)
	title.SetRect(termWidth/2, currentTop, termWidth, currentTop+height)
	ui.Render(title)
	currentTop = currentTop + height + 1

	message := widgets.NewParagraph()
	// WrapText is set to false to prevent internal wordwrap which is not accounting for the padding
	message.WrapText = false
	// For long text that lead to wrapping text, the terminal width is divided by 2 and deducted by MESSAGE_TEXT_PADDING to account for the padding
	message.Text = wordwrap.WrapString(analysisResult.Message, uint(termWidth/2-constants.MESSAGE_TEXT_PADDING))
	if analysisResult.URI != "" {
		// Add URL to the message with wordwrap
		// Add emply lines as separator
		urlText := wordwrap.WrapString(fmt.Sprintf("For more information: %s", analysisResult.URI), uint(termWidth/2-constants.MESSAGE_TEXT_PADDING))
		message.Text = message.Text + "\n\n" + urlText
	}
	height = util.EstimateNumberOfLines(message.Text) + constants.MESSAGE_TEXT_LINES_MARGIN_TO_BOTTOM
	message.Border = false
	message.SetRect(termWidth/2, currentTop, termWidth, currentTop+height)
	ui.Render(message)
}

func showSaved(filename string) {
	termWidth, termHeight := ui.TerminalDimensions()

	savedMessage := widgets.NewParagraph()
	savedMessage.Text = fmt.Sprintf("Preflight results saved to\n\n%s", filename)
	savedMessage.WrapText = true
	savedMessage.Border = true

	left := termWidth/2 - 20
	right := termWidth/2 + 20
	top := termHeight/2 - 4
	bottom := termHeight/2 + 4

	savedMessage.SetRect(left, top, right, bottom)
	ui.Render(savedMessage)

	isShowingSaved = true
}
