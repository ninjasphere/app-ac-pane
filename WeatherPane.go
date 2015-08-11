package main

import (
	"image"

	"github.com/ninjasphere/gestic-tools/go-gestic-sdk"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/sphere-go-led-controller/util"
	"github.com/ninjasphere/usvc-lib/config"
)

var enableACPane = config.Bool(true, "ac.enable")

type ACPane struct {
	image util.Image
}

func NewACPane(conn *ninja.Connection) *ACPane {

	pane := &ACPane{
		image: util.LoadImage(util.ResolveImagePath("fan-on-blue.gif")),
	}

	return pane
}

func (p *ACPane) KeepAwake() bool {
	return false
}

func (p *ACPane) IsEnabled() bool {
	return enableACPane
}

func (p *ACPane) Gesture(gesture *gestic.GestureMessage) {
	if gesture.Tap.Active() {
		log.Infof("AC tap!")
	}
}

func (p *ACPane) Render() (*image.RGBA, error) {
	/*if p.temperature {
		img := image.NewRGBA(image.Rect(0, 0, 16, 16))

		drawText := func(text string, col color.RGBA, top int) {
			width := O4b03b.Font.DrawString(img, 0, 8, text, color.Black)
			start := int(16 - width - 1)

			//spew.Dump("text", text, "width", width, "start", start)

			O4b03b.Font.DrawString(img, start, top, text, col)
		}

		today := p.forecast.Daily.Data[0]

		var min, max string
		if p.forecast.Flags.Units == "us" {
			min = fmt.Sprintf("%dF", int(today.TemperatureMin))
			max = fmt.Sprintf("%dF", int(today.TemperatureMax))
		} else {
			min = fmt.Sprintf("%dC", int(today.TemperatureMin))
			max = fmt.Sprintf("%dC", int(today.TemperatureMax))
		}

		drawText(max, color.RGBA{253, 151, 32, 255}, 3)
		drawText(min, color.RGBA{69, 175, 249, 255}, 10)

		return img, nil
	} else {*/
	return p.image.GetNextFrame(), nil
}

func (p *ACPane) IsDirty() bool {
	return true
}
