package charts

import (
	"bytes"
	"fmt"
	"image/color"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type AssetData struct {
	Name  string
	Value float64
}

type HistoryPoint struct {
	Date  time.Time
	Total float64
}

func RenderAllocationBar(assets []AssetData) ([]byte, error) {
	p := plot.New()
	p.Title.Text = "资产配比"
	p.Title.Padding = vg.Points(8)
	p.Y.Min = 0

	values := make(plotter.Values, len(assets))
	for i, a := range assets {
		values[i] = a.Value
	}

	bar, err := plotter.NewBarChart(values, vg.Points(50))
	if err != nil {
		return nil, fmt.Errorf("failed to create bar chart: %w", err)
	}
	bar.Color = color.RGBA{R: 26, G: 26, B: 26, A: 255}
	bar.LineStyle.Width = vg.Length(0)

	p.Add(bar)

	names := make([]string, len(assets))
	for i, a := range assets {
		names[i] = a.Name
	}
	p.NominalX(names...)

	w, h := 5*vg.Inch, 3*vg.Inch
	wt, err := p.WriterTo(w, h, "png")
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	var buf bytes.Buffer
	if _, err := wt.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("failed to write chart: %w", err)
	}

	return buf.Bytes(), nil
}

func RenderValueLine(points []HistoryPoint) ([]byte, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	p := plot.New()
	p.Title.Text = "组合净值走势"
	p.Title.Padding = vg.Points(8)
	p.X.Tick.Marker = plot.TimeTicks{Format: "01-02"}

	pts := make(plotter.XYs, len(points))
	for i, pt := range points {
		pts[i].X = float64(pt.Date.Unix())
		pts[i].Y = pt.Total
	}

	err := plotutil.AddLinePoints(p, pts)
	if err != nil {
		return nil, fmt.Errorf("failed to add line: %w", err)
	}

	w, h := 5*vg.Inch, 2.5*vg.Inch
	wt, err := p.WriterTo(w, h, "png")
	if err != nil {
		return nil, fmt.Errorf("failed to create writer: %w", err)
	}

	var buf bytes.Buffer
	if _, err := wt.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("failed to write chart: %w", err)
	}

	return buf.Bytes(), nil
}
