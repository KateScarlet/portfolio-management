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

var assetColors = []color.Color{
	color.RGBA{R: 26, G: 26, B: 26, A: 255},    // 股票 - 黑色
	color.RGBA{R: 134, G: 142, B: 150, A: 255}, // 债券 - 灰色
	color.RGBA{R: 180, G: 190, B: 200, A: 255}, // 现金 - 浅灰
	color.RGBA{R: 212, G: 175, B: 55, A: 255},  // 商品 - 金色
}

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
	p.Title.Padding = vg.Points(12)
	p.X.Padding = vg.Points(20)
	p.Y.Padding = vg.Points(10)
	p.Y.Min = 0

	total := 0.0
	for _, a := range assets {
		total += a.Value
	}
	if total == 0 {
		return nil, fmt.Errorf("total value is zero")
	}

	for i, a := range assets {
		if a.Value == 0 {
			continue
		}
		vals := plotter.Values{a.Value}
		bar, err := plotter.NewBarChart(vals, vg.Points(60))
		if err != nil {
			return nil, fmt.Errorf("failed to create bar: %w", err)
		}
		bar.Color = assetColors[i%len(assetColors)]
		bar.LineStyle.Width = vg.Length(0)
		bar.XMin = float64(i)
		p.Add(bar)
	}

	names := make([]string, len(assets))
	for i, a := range assets {
		pct := a.Value / total * 100
		names[i] = fmt.Sprintf("%s %.0f%%", a.Name, pct)
	}
	p.NominalX(names...)

	w, h := 5*vg.Inch, 3.2*vg.Inch
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
