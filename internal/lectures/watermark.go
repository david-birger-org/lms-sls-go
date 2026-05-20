package lectures

import (
	"bytes"
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

const watermarkText = "David Birger"
const watermarkDesc = "font:Helvetica, points:32, color:0.6 0.6 0.6, opacity:0.08, rotation:-45, scalefactor:1.0 abs, position:c"

func ApplyWatermark(pdfBytes []byte) ([]byte, error) {
	conf := model.NewDefaultConfiguration()
	wm, err := api.TextWatermark(watermarkText, watermarkDesc, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("build watermark: %w", err)
	}

	var out bytes.Buffer
	if err := api.AddWatermarks(bytes.NewReader(pdfBytes), &out, nil, wm, conf); err != nil {
		return nil, fmt.Errorf("apply watermark: %w", err)
	}
	return out.Bytes(), nil
}
