package lectures

import (
	"bytes"
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const (
	watermarkText = "David Birger"
	watermarkDesc = "font:Helvetica, points:32, color:0.6 0.6 0.6, opacity:0.08, rotation:-45, scale:1 abs, diagonal:1, mode:watermark, position:c"
)

func ApplyWatermark(pdf []byte) ([]byte, error) {
	in := bytes.NewReader(pdf)
	out := new(bytes.Buffer)

	wm, err := api.TextWatermark(watermarkText, watermarkDesc, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("build watermark: %w", err)
	}

	if err := api.AddWatermarks(in, out, nil, wm, nil); err != nil {
		return nil, fmt.Errorf("apply watermark: %w", err)
	}
	return out.Bytes(), nil
}
