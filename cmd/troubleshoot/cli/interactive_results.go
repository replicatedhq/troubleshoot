package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/cmd/util"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

var (
	selectedResult = 0
	isShowingSaved = false
)

func showInteractiveResults(analyzeResults []*analyzerunner.AnalyzeResult) error {
	if err := ui.Init(); err != nil {
		return errors.Wrap(err, "failed to create terminal ui")
	}
	defer ui.Close()
	drawUI(analyzeResults)

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
					drawUI(analyzeResults)
				} else {
					return nil
				}
			case "s":
				filename, err := save(analyzeResults)
				if err != nil {
					// show
				} else {
					showSaved(filename)
					go func() {
						time.Sleep(time.Second * 5)
						isShowingSaved = false
						ui.Clear()
						drawUI(analyzeResults)
					}()
				}
			case "<Resize>":
				ui.Clear()
				drawUI(analyzeResults)
			case "<Down>":
				if selectedResult < len(analyzeResults)-1 {
					selectedResult++
				} else {
					selectedResult = 0
				}
				ui.Clear()
				drawUI(analyzeResults)
			case "<Up>":
				if selectedResult > 0 {
					selectedResult--
				} else {
					selectedResult = len(analyzeResults) - 1
				}
				ui.Clear()
				drawUI(analyzeResults)
			}
		}
	}
}

func drawUI(analyzeResults []*analyzerunner.AnalyzeResult) {
	drawGrid(analyzeResults)
	drawFooter()
}

func drawGrid(analyzeResults []*analyzerunner.AnalyzeResult) {
	termWidth, _ := ui.TerminalDimensions()

	tileWidth := 40
	tileHeight := 10

	columnCount := termWidth / tileWidth

	row := 0
	col := 0

	for _, analyzeResult := range analyzeResults {
		// draw this file

		tile := widgets.NewParagraph()
		tile.Title = analyzeResult.Title
		tile.Text = analyzeResult.Message
		tile.PaddingLeft = 1
		tile.PaddingBottom = 1
		tile.PaddingRight = 1
		tile.PaddingTop = 1

		tile.SetRect(col*tileWidth, row*tileHeight, col*tileWidth+tileWidth, row*tileHeight+tileHeight)

		if analyzeResult.IsFail {
			tile.BorderStyle.Fg = ui.ColorRed
		} else if analyzeResult.IsWarn {
			tile.BorderStyle.Fg = ui.ColorYellow
		} else {
			tile.BorderStyle.Fg = ui.ColorGreen
		}

		ui.Render(tile)

		col++

		if col >= columnCount {
			col = 0
			row++
		}
	}
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
