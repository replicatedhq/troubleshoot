package cli

import (
	"fmt"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	analyzerunner "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

type nodeValue string

func (nv nodeValue) String() string {
	return string(nv)
}

func showInteractiveResults(analyzeResults []*analyzerunner.AnalyzeResult) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	selectedResult := 0

	preflightTable := getPreflightTable(analyzeResults)
	details := getDetails(analyzeResults[selectedResult])

	grid := ui.NewGrid()
	termWidth, termHeight := ui.TerminalDimensions()
	grid.SetRect(0, 0, termWidth, termHeight)

	grid.Set(
		ui.NewRow(1.0,
			ui.NewCol(1.0/2, preflightTable),
			ui.NewCol(1.0/2, details),
		),
	)

	ui.Render(grid)

	uiEvents := ui.PollEvents()
	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return nil
			case "<Resize>":
				payload := e.Payload.(ui.Resize)
				grid.SetRect(0, 0, payload.Width, payload.Height)
				ui.Clear()
				ui.Render(grid)
			}
		}
	}
}

func getPreflightTable(analyzeResults []*analyzerunner.AnalyzeResult) *widgets.Table {
	table := widgets.NewTable()
	table.Border = true
	table.Rows = [][]string{}

	for i, analyzeResult := range analyzeResults {
		table.Rows = append(table.Rows, []string{
			analyzeResult.Title,
		})

		if analyzeResult.IsPass {
			table.RowStyles[i] = ui.NewStyle(ui.ColorGreen, ui.ColorClear, ui.ModifierBold)
		} else if analyzeResult.IsWarn {
			table.RowStyles[i] = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)
		} else if analyzeResult.IsFail {
			table.RowStyles[i] = ui.NewStyle(ui.ColorRed, ui.ColorClear)
		}
	}

	return table
}

func getDetails(analysisResult *analyzerunner.AnalyzeResult) *ui.Grid {
	grid := ui.NewGrid()

	entries := []interface{}{}

	title := widgets.NewParagraph()
	title.Text = analysisResult.Title
	title.Border = false
	entries = append(entries, ui.NewRow(0.2, ui.NewCol(1.0, title)))

	message := widgets.NewParagraph()
	message.Text = analysisResult.Message
	message.Border = false
	entries = append(entries, ui.NewRow(0.2, ui.NewCol(1.0, message)))

	if analysisResult.URI != "" {
		uri := widgets.NewParagraph()
		uri.Text = fmt.Sprintf("For more information: %s", analysisResult.URI)
		uri.Border = false
		entries = append(entries, ui.NewRow(0.2, ui.NewCol(1.0, uri)))
	}

	grid.Set(entries...)
	return grid
}
