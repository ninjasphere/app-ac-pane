package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"time"

	// "github.com/davecgh/go-spew/spew"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/ninjasphere/gestic-tools/go-gestic-sdk"
	"github.com/ninjasphere/go-ninja/api"
	"github.com/ninjasphere/go-ninja/channels"
	"github.com/ninjasphere/go-ninja/model"
	"github.com/ninjasphere/sphere-go-led-controller/fonts/clock"
	"github.com/ninjasphere/sphere-go-led-controller/ui"
	"github.com/ninjasphere/sphere-go-led-controller/util"
	"github.com/ninjasphere/usvc-lib/config"
)

var enableACPane = config.Bool(true, "ac.enable")
var tempAdjustInterval = config.Duration(time.Millisecond*300, "ac.airwheel.interval")
var airWheelReset = config.Duration(time.Millisecond*500, "ac.airwheel.reset")
var tapTimeout = config.Duration(time.Millisecond*500, "ac.tap.timeout")

var (
	red, _   = colorful.Hex("#fd9720")
	blue, _  = colorful.Hex("#45aff9")
	green, _ = colorful.Hex("#45f96b")
)

type ACPane struct {
	mode        string
	images      map[string]util.Image
	targetTemp  int
	currentTemp int
	flash       bool

	thermostat *ninja.ServiceClient
	acstat     *ninja.ServiceClient

	lastTempAdjust   time.Time
	lastAirWheelTime time.Time
	lastAirWheel     *uint8
	countSinceLast   int
	ignoringTap      bool
	ignoreTapTimer   *time.Timer
}

func NewACPane(conn *ninja.Connection) *ACPane {

	pane := &ACPane{
		mode:        "off",
		flash:       false,
		targetTemp:  666,
		currentTemp: 666,
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

	listening := make(map[string]bool)

	isFlash := func(s string) bool {
		return s == "SUSPENDED"
	}

	onState := func(protocol, event string, cb func(params *json.RawMessage)) {
		ui.GetChannelServicesContinuous("aircon", protocol, func(thing *model.Thing) bool {
			return true
		}, func(devices []*ninja.ServiceClient, err error) {
			if err != nil {
				log.Infof("Failed to update %s device: %s", protocol, err)
			} else {
				log.Infof("Got %d %s devices", len(devices), protocol)

				for _, device := range devices {

					log.Debugf("Checking %s device %s", protocol, device.Topic)

					if _, ok := listening[device.Topic]; !ok {

						// New device
						log.Infof("Got new %s device: %s", protocol, device.Topic)

						if protocol == "thermostat" {
							pane.thermostat = device
						}

						if protocol == "acstat" {
							pane.acstat = device
						}

						if protocol == "demand" {

							// only need to query for the initial notification - listener will pick up the rest

							go func() {
								var state StateChangeNotification
								err := device.Call("getControlState", nil, &state, time.Second*10)
								if err != nil {
									log.Errorf("Failed to fetch state from demand channel: %s", err)
								}

								// spew.Dump("Got demand state", state)

								pane.flash = isFlash(state.State)
							}()
						}

						listening[device.Topic] = true

						device.OnEvent(event, func(params *json.RawMessage, values map[string]string) bool {
							cb(params)

							return true
						})
					}
				}
			}
		})
	}

	onState("temperature", "state", func(params *json.RawMessage) {
		var temp float64
		err := json.Unmarshal(*params, &temp)
		if err != nil {
			log.Infof("Failed to unmarshal temp from %s error:%s", *params, err)
		}

		pane.currentTemp = int(temp)

		log.Infof("Got the temp %d", pane.currentTemp)
	})

	onState("thermostat", "state", func(params *json.RawMessage) {
		var temp float64
		err := json.Unmarshal(*params, &temp)
		if err != nil {
			log.Infof("Failed to unmarshal thermostat from %s error:%s", *params, err)
		}

		pane.targetTemp = int(temp)

		log.Infof("Got the thermostat %d", pane.targetTemp)
	})

	onState("acstat", "state", func(params *json.RawMessage) {
		var state channels.ACState
		err := json.Unmarshal(*params, &state)
		if err != nil {
			log.Infof("Failed to unmarshal acstat from %s error:%s", *params, err)
		}

		pane.mode = *state.Mode

		log.Infof("Got the ac mode %d", pane.mode)
	})

	onState("demand", "controlstate", func(params *json.RawMessage) {

		// spew.Dump("demand/controlstate", params)

		var state StateChangeNotification
		err := json.Unmarshal(*params, &state)
		if err != nil {
			log.Infof("Failed to unmarshal demandstat state from %s error:%s", *params, err)
		}

		pane.flash = isFlash(state.State)

		log.Infof("Got the demandstat state %d", pane.mode)
	})

	go ui.StartSearchTasks(conn)

	return pane
}

type StateChangeNotification struct {
	State string `json:"state"` // the current state of the controller
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

	if p.acstat != nil && !p.ignoringTap && gesture.Tap.Active() {
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

		p.acstat.Call("set", channels.ACState{
			Mode: &p.mode,
		}, nil, 0)

	}

	if p.thermostat != nil && p.targetTemp != 666 && p.lastAirWheel == nil || gesture.AirWheel.Counter != int(*p.lastAirWheel) {

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

				p.thermostat.Call("set", p.targetTemp, nil, 0)

			}

		}

		val := uint8(gesture.AirWheel.Counter)
		p.lastAirWheel = &val
	}
}

func (p *ACPane) Render() (*image.RGBA, error) {

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))

	drawText := func(text string, col color.Color, x, y int) {
		clock.Font.DrawString(img, x, y, text, col)
	}

	draw.Draw(img, img.Bounds(), p.images[p.mode].GetNextFrame(), img.Bounds().Min, draw.Src)

	targetCol := green
	if p.flash {
		//drawText("$", green, 0, 11)
		h, s, v := targetCol.Hsv()
		targetCol = colorful.Hsv(sinTime(h), s, v)
	}

	if p.currentTemp != 666 {

		if p.currentTemp > p.targetTemp {
			drawText(fmt.Sprintf("%02d", p.currentTemp), red, 0, 11)
		} else if p.currentTemp < p.targetTemp {
			drawText(fmt.Sprintf("%02d", p.currentTemp), blue, 0, 11)
		}

	}

	if p.targetTemp != 666 {
		drawText(fmt.Sprintf("%02d", p.targetTemp), targetCol, 9, 11)
	}

	return img, nil
}

func (p *ACPane) IsDirty() bool {
	return true
}

func sinTime(x float64) float64 {
	t := float64(time.Now().Nanosecond()/int(time.Millisecond)) / 1000.0
	t = math.Sin((t - 0.5) * (2 * math.Pi))
	t = (t + 1) / 2
	return x * t
}
