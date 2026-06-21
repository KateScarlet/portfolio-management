package charts

import (
	"bytes"
	"fmt"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type HistoryPoint struct {
	Date  time.Time
	Total float64
}

func RenderValueLine(points []HistoryPoint) ([]byte, error) {
	if len(points) == 0 {
		return nil, fmt.Errorf("no data points provided")
	}

	p := plot.New()
	p.Title.Text = "组合净值走势"
	p.Title.Padding = vg.Points(12)
	p.X.Padding = vg.Points(10)
	p.Y.Padding = vg.Points(10)
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

	w, h := 5*vg.Inch, 2.8*vg.Inch
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
