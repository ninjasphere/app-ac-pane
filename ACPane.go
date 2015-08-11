package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"github.com/ninjasphere/gestic-tools/go-gestic-sdk"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/sphere-go-led-controller/fonts/O4b03b"
	"github.com/ninjasphere/sphere-go-led-controller/util"
	"github.com/ninjasphere/usvc-lib/config"
)

var enableACPane = config.Bool(true, "ac.enable")
var tempAdjustInterval = config.Duration(time.Millisecond*300, "ac.airwheel.interval")
var airWheelReset = config.Duration(time.Millisecond*500, "ac.airwheel.reset")
var tapTimeout = config.Duration(time.Millisecond*500, "ac.tap.timeout")

type ACPane struct {
	mode       string
	images     map[string]util.Image
	targetTemp int

	lastTempAdjust   time.Time
	lastAirWheelTime time.Time
	lastAirWheel     *uint8
	countSinceLast   int
	ignoringTap      bool
	ignoreTapTimer   *time.Timer
}

func NewACPane(conn *ninja.Connection) *ACPane {

	pane := &ACPane{
		mode:       "off",
		targetTemp: 22,
		images: map[string]util.Image{
			"off":  util.LoadImage(util.ResolveImagePath("fan.gif")),
			"fan":  util.LoadImage(util.ResolveImagePath("fan-on.gif")),
			"cool": util.LoadImage(util.ResolveImagePath("fan-on-blue.gif")),
			"heat": util.LoadImage(util.ResolveImagePath("fan-on-red.gif")),
		},
	}

	pane.ignoreTapTimer = time.AfterFunc(0, func() {
		pane.ignoringTap = false
	})

	return pane
}

func (p *ACPane) KeepAwake() bool {
	return true
}

func (p *ACPane) Locked() bool {
	return true
}

func (p *ACPane) IsEnabled() bool {
	return enableACPane
}

func (p *ACPane) Gesture(gesture *gestic.GestureMessage) {

	if !p.ignoringTap && gesture.Tap.Active() {
		log.Infof("AC tap!")

		p.ignoringTap = true
		p.ignoreTapTimer.Reset(tapTimeout)

		switch p.mode {
		case "off":
			p.mode = "fan"
		case "fan":
			p.mode = "cool"
		case "cool":
			p.mode = "heat"
		case "heat":
			p.mode = "off"
		}
	}

	if p.lastAirWheel == nil || gesture.AirWheel.Counter != int(*p.lastAirWheel) {

		if time.Since(p.lastAirWheelTime) > airWheelReset {
			p.lastAirWheel = nil
		}

		if p.countSinceLast > gesture.AirWheel.CountSinceLast {
			p.lastAirWheel = nil
		}

		p.countSinceLast = gesture.AirWheel.CountSinceLast

		p.lastAirWheelTime = time.Now()

		log.Debugf("Airwheel: %d", gesture.AirWheel.Counter)

		if p.lastAirWheel != nil {
			offset := int(gesture.AirWheel.Counter) - int(*p.lastAirWheel)

			if offset > 30 {
				offset -= 255
			}

			if offset < -30 {
				offset += 255
			}

			log.Debugf("Airwheel New: %d Offset: %d Last: %d", gesture.AirWheel.Counter, offset, *p.lastAirWheel)

			log.Debugf("Current temp %f", p.targetTemp)

			log.Debugf("Temp offset %f", float64(offset)/255.0)

			if time.Since(p.lastTempAdjust) < tempAdjustInterval {
				log.Debugf("Temp rate limited")
			} else {
				p.lastTempAdjust = time.Now()
				log.Debugf("Temp NOT rate limited")
				if offset > 0 {
					p.targetTemp += 1
				} else {
					p.targetTemp -= 1
				}

			}

		}

		val := uint8(gesture.AirWheel.Counter)
		p.lastAirWheel = &val
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

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	drawText := func(text string, col color.RGBA, x, y int) {
		//width := O4b03b.Font.DrawString(img, 0, 8, text, color.Black)
		///start := int(16 - width - 1)

		//spew.Dump("text", text, "width", width, "start", start)

		O4b03b.Font.DrawString(img, x, y, text, col)
	}

	draw.Draw(img, img.Bounds(), p.images[p.mode].GetNextFrame(), img.Bounds().Min, draw.Src)

	drawText("24", color.RGBA{253, 151, 32, 255}, 0, 11)
	drawText(fmt.Sprintf("%d", p.targetTemp), color.RGBA{69, 175, 249, 255}, 9, 11)

	return img, nil
}

func (p *ACPane) IsDirty() bool {
	return true
}
