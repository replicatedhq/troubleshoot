package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
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

func showInteractiveResults(supportBundleName string, analyzeResults []*analyzerunner.AnalyzeResult, archivePath string) error {
	if err := ui.Init(); err != nil {
		return errors.Wrap(err, "failed to create terminal ui")
	}
	defer ui.Close()
	drawUI(supportBundleName, analyzeResults)

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
					drawUI(supportBundleName, analyzeResults)
				} else {
					return nil
				}
			case "s":
				filename, err := save(analyzeResults)
				if err != nil {
					// show
				} else {
					showSaved(filename, archivePath)
					go func() {
						time.Sleep(time.Second * 5)
						isShowingSaved = false
						ui.Clear()
						drawUI(supportBundleName, analyzeResults)
					}()
				}
			case "<Resize>":
				ui.Clear()
				drawUI(supportBundleName, analyzeResults)
			case "<Down>":
				if selectedResult < len(analyzeResults)-1 {
					selectedResult++
				} else {
					selectedResult = 0
					table.SelectedRow = 0
				}
				table.ScrollDown()
				ui.Clear()
				drawUI(supportBundleName, analyzeResults)
			case "<Up>":
				if selectedResult > 0 {
					selectedResult--
				} else {
					selectedResult = len(analyzeResults) - 1
					table.SelectedRow = len(analyzeResults)
				}
				table.ScrollUp()
				ui.Clear()
				drawUI(supportBundleName, analyzeResults)
			}
		}
	}
}

func drawUI(supportBundleName string, analyzeResults []*analyzerunner.AnalyzeResult) {
	drawGrid(analyzeResults)
	drawHeader(supportBundleName)
	drawFooter()
}

func drawGrid(analyzeResults []*analyzerunner.AnalyzeResult) {
	drawAnalyzersTable(analyzeResults)
	drawDetails(analyzeResults[selectedResult])
}

func drawHeader(supportBundleName string) {
	termWidth, _ := ui.TerminalDimensions()

	title := widgets.NewParagraph()
	title.Text = fmt.Sprintf("%s Support Bundle Analysis", util.AppName(supportBundleName))
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

func drawAnalyzersTable(analyzeResults []*analyzerunner.AnalyzeResult) {
	termWidth, termHeight := ui.TerminalDimensions()

	table.SetRect(0, 3, termWidth/2, termHeight-6)
	table.FillRow = true
	table.Border = true
	table.Rows = [][]string{}
	table.ColumnWidths = []int{termWidth}

	for i, analyzeResult := range analyzeResults {
		title := analyzeResult.Title
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

func showSaved(filename string, archivePath string) {
	termWidth, termHeight := ui.TerminalDimensions()

	f := `A support bundle was generated and saved at %s. 
Please send this file to your software vendor for support.`
	additionalMessageText := fmt.Sprintf(f, archivePath)

	savedMessage := widgets.NewParagraph()
	savedMessage.Text = fmt.Sprintf("Support Bundle analysis results saved to\n\n%s\n\n%s", filename, additionalMessageText)
	savedMessage.WrapText = true
	savedMessage.Border = true

	// Split the text into lines and find the longest line
	lines := strings.Split(savedMessage.Text, "\n")
	maxLineLength := 0
	for _, line := range lines {
		if len(line) > maxLineLength {
			// maxLineLength is set to half of the line length to prevent the showing text with more space than needed
			maxLineLength = len(line)/2 + constants.MESSAGE_TEXT_PADDING
		}
	}

	if maxLineLength > termWidth/2 {
		maxLineLength = termWidth / 2
	}

	left := termWidth/2 - maxLineLength
	right := termWidth/2 + maxLineLength
	top := termHeight/2 - len(lines)
	bottom := termHeight/2 + len(lines)

	savedMessage.SetRect(left, top, right, bottom)
	ui.Render(savedMessage)

	isShowingSaved = true
}

func save(analyzeResults []*analyzerunner.AnalyzeResult) (string, error) {
	filename := path.Join(util.HomeDir(), fmt.Sprintf("%s-results.txt", "support-bundle"))
	_, err := os.Stat(filename)
	if err == nil {
		os.Remove(filename)
	}

	results := ""
	for _, analyzeResult := range analyzeResults {
		result := ""

		if analyzeResult.IsPass {
			result = "Check PASS\n"
		} else if analyzeResult.IsWarn {
			result = "Check WARN\n"
		} else if analyzeResult.IsFail {
			result = "Check FAIL\n"
		}

		result = result + fmt.Sprintf("Title: %s\n", analyzeResult.Title)
		result = result + fmt.Sprintf("Message: %s\n", analyzeResult.Message)

		if analyzeResult.URI != "" {
			result = result + fmt.Sprintf("URI: %s\n", analyzeResult.URI)
		}

		result = result + "\n------------\n"

		results = results + result
	}

	if err := ioutil.WriteFile(filename, []byte(results), 0644); err != nil {
		return "", errors.Wrap(err, "failed to save preflight results")
	}

	return filename, nil
}
