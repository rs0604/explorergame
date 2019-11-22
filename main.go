package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/align"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/termbox"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/button"
	"github.com/mum4k/termdash/widgets/donut"
	"github.com/mum4k/termdash/widgets/gauge"
	"github.com/mum4k/termdash/widgets/segmentdisplay"
	"github.com/mum4k/termdash/widgets/text"
)

// 3次元の座標データ
type Point3D struct {
	x float64
	y float64
	z float64
}

// プレイヤーデータ
type Player struct {
	// 現在位置
	position Point3D

	// タービン回転数： 0 ~ 200
	turbineRpmSettingValue float64

	turbineRpmActualValue float64

	// 現在の速度
	velocity float64

	// 加速度
	acceleration float64

	// 舵の角度 -35 ~ 35
	rudderAngle float64

	// 船が向いている方角
	direction float64

	// 転回の勢い
	directionAcceleration float64

	// 浮力： 0.0 ~ 100.0
	buoyancy float64

	// 浮力によって生じる加速度
	buoyancyAcceleration float64
}

var debug bool = true

func debugLog(message string) {
	if debug {
		fmt.Println(message)
	}
}

func writeLines(ctx context.Context, p *Player, t *text.Text, delay time.Duration) {
	var message = ""
	if p.velocity < 1.0 {
		message = "Stopped." + strconv.FormatFloat(p.velocity, 'f', 4, 64)
	} else if p.velocity < 10.0 {
		message = "Nearly Stopped." + strconv.FormatFloat(p.velocity, 'f', 4, 64)
	} else if p.velocity < 50.0 {
		message = "Moving forward at low speed." + strconv.FormatFloat(p.velocity, 'f', 4, 64)
	} else if p.velocity < 100.0 {
		message = "Moving forward." + strconv.FormatFloat(p.velocity, 'f', 4, 64)
	} else if p.velocity < 150.0 {
		message = "Moving forward at high speed." + strconv.FormatFloat(p.velocity, 'f', 4, 64)
	} else {
		message = "Full speed forward."
	}
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := t.Write(fmt.Sprintf("%s\n", message)); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func updateTick(ctx context.Context, p *Player, display *segmentdisplay.SegmentDisplay, delay time.Duration) {
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:

			// 速度の更新 --------------------------------------------------------------------------------
			// 回転数の計算
			p.turbineRpmActualValue += (p.turbineRpmSettingValue - p.turbineRpmActualValue) / ((p.turbineRpmActualValue + 1) * 5)
			p.turbineRpmActualValue *= 0.998
			p.turbineRpmActualValue += p.turbineRpmActualValue * rand.Float64() * 0.004

			// 加速度の計算
			p.acceleration = float64(p.turbineRpmActualValue / 10.0)

			// 速度の計算
			p.velocity += p.acceleration / 10
			p.velocity *= 0.99 + rand.Float64()*0.003 // 減速係数
			if err := display.Write([]*segmentdisplay.TextChunk{
				segmentdisplay.NewChunk(fmt.Sprintf("%06.1f", p.velocity)),
			}); err != nil {
				panic(err)
			}

		case <-ctx.Done():
			return
		}
	}
}

// タービン回転数設定値ゲージ
func rpmSettingGauge(ctx context.Context, p *Player, g *gauge.Gauge, delay time.Duration) {
	ticker := time.NewTicker(delay)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			displayValue := int(math.Max(math.Min(float64(p.turbineRpmSettingValue), 200.0), 0))
			if err := g.Absolute(displayValue, 200); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// タービン回転数ゲージ
func rpmMeterDonut(ctx context.Context, p *Player, d *donut.Donut, delay time.Duration) {
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			displayValue := math.Max(math.Min(float64(p.turbineRpmActualValue), 200.0), 0)

			if displayValue < 140 {
				if err := d.Absolute(int(displayValue), 200, donut.CellOpts(cell.FgColor(cell.ColorYellow))); err != nil {
					panic(err)
				}
			} else {
				if err := d.Absolute(int(displayValue), 200, donut.CellOpts(cell.FgColor(cell.ColorRed))); err != nil {
					panic(err)
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// 舵の角度
func rudderAngleGauge(ctx context.Context, p *Player, g *gauge.Gauge, delay time.Duration) {
	ticker := time.NewTicker(delay)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			displayValue := int(math.Max(math.Min(float64(p.rudderAngle), 70.0), 0))
			if err := g.Absolute(displayValue, 70); err != nil {
				panic(err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	debugLog("main(): start")
	// プレイヤーの状態初期化

	player := Player{
		position:               Point3D{x: 0.0, y: 0.0, z: 0.0},
		turbineRpmSettingValue: 0,
		turbineRpmActualValue:  0,
		velocity:               0.0,
		acceleration:           0.0,
		rudderAngle:            35.0,
		direction:              0.0,
		directionAcceleration:  0.0,
		buoyancy:               50.0,
		buoyancyAcceleration:   0.0,
	}

	t, err := termbox.New()
	if err != nil {
		panic(err)
	}
	defer t.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// segment display
	display, err := segmentdisplay.New()
	if err != nil {
		panic(err)
	}

	if err := display.Write([]*segmentdisplay.TextChunk{
		segmentdisplay.NewChunk(fmt.Sprintf("%06.1f", player.velocity)),
	}); err != nil {
		panic(err)
	}

	// 左下メッセージ
	wrapped, err := text.New(text.WrapAtRunes())
	if err != nil {
		panic(err)
	}
	if err := wrapped.Write("Internal Pressure: 1024 mpa\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("External Pressure: 42821 mpa\n", text.WriteCellOpts(cell.FgColor(cell.ColorYellow))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("\nReactor Temp: 3081 K\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("Fuel: 102241\n", text.WriteCellOpts(cell.FgColor(cell.ColorBlue))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("Turbine rpm: 328\n", text.WriteCellOpts(cell.FgColor(cell.ColorBlue))); err != nil {
		panic(err)
	}

	if err := wrapped.Write("\nCurrent Direction: 248° [SWW]\n", text.WriteCellOpts(cell.FgColor(cell.ColorCyan))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("Altitude: -12832 ft. \n", text.WriteCellOpts(cell.FgColor(cell.ColorCyan))); err != nil {
		panic(err)
	}

	if err := wrapped.Write("\nIrradiated rader strength: 0\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("Sonar ping Effectiveness: 76%\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("Threat Level: Green\n", text.WriteCellOpts(cell.FgColor(cell.ColorGreen))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("\n[Weapon] Torpedo: 11\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("[Weapon] Surface-t-air Missile: 11\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}
	if err := wrapped.Write("[Weapon] UAV: 3\n", text.WriteCellOpts(cell.FgColor(cell.ColorRed))); err != nil {
		panic(err)
	}

	// rolled
	rolled, err := text.New(text.RollContent(), text.WrapAtWords())
	if err != nil {
		panic(err)
	}
	if err := rolled.Write("<< Rolls the content upwards if RollContent() option is provided. >>\n"); err != nil {
		panic(err)
	}

	// 速度関連
	buttonTurbinePlus, err := button.New("+ 10", func() error {
		// process
		player.turbineRpmSettingValue += 10
		if player.turbineRpmSettingValue > 200 {
			player.turbineRpmSettingValue = 200
		}
		return display.Write([]*segmentdisplay.TextChunk{
			segmentdisplay.NewChunk(fmt.Sprintf("%06.1f", player.velocity)),
		})
	})

	buttonTurbineMinus, err := button.New("- 10", func() error {
		// process
		player.turbineRpmSettingValue -= 10
		if player.turbineRpmSettingValue < 0 {
			player.turbineRpmSettingValue = 0
		}
		return display.Write([]*segmentdisplay.TextChunk{
			segmentdisplay.NewChunk(fmt.Sprintf("%06.1f", player.velocity)),
		})
	})

	rpmMeter, err := donut.New(
		donut.CellOpts(cell.FgColor(cell.ColorYellow)),
		donut.HolePercent(50),
		donut.ShowTextProgress(),
		donut.Label("turbine rpm", cell.FgColor(cell.ColorYellow)),
	)
	if err != nil {
		panic(err)
	}

	rpmSettingMeter, err := gauge.New(
		gauge.Height(1),
		gauge.Border(linestyle.Light),
		gauge.BorderTitle("Setting Value"),
	)
	if err != nil {
		panic(err)
	}

	// 転回関連
	rudderAngleGaugeObj, err := gauge.New(
		gauge.Color(cell.ColorRed),
		gauge.Height(1),
		gauge.Border(linestyle.Light, cell.FgColor(cell.ColorYellow)),
		gauge.BorderTitle("<== L == | == R ==>"),
		gauge.BorderTitleAlign(align.HorizontalCenter),
		gauge.HideTextProgress(),
	)

	rudderLeftButtonObj, err := button.New("L", func() error {
		settingValue := player.rudderAngle - 2.5

		player.rudderAngle = math.Max(math.Min(player.rudderAngle-2.5, 70), 0)
	})

	go rpmMeterDonut(ctx, &player, rpmMeter, 100*time.Millisecond)
	go rpmSettingGauge(ctx, &player, rpmSettingMeter, 250*time.Millisecond)
	go updateTick(ctx, &player, display, 16*time.Millisecond)
	go rudderAngleGauge(ctx, &player, rudderAngleGaugeObj, 16*time.Millisecond)

	// Layout ----------------------------------------------------------------------
	go writeLines(ctx, &player, rolled, 1*time.Second)
	c, err := container.New(
		t,
		container.Border(linestyle.Light),
		container.BorderTitle("PRESS Q TO QUIT"),
		container.SplitVertical(
			container.Left(
				container.SplitHorizontal(
					container.Top(
						container.SplitHorizontal(
							container.Top(
								container.Border(linestyle.Light),
								container.BorderTitle("Current Speed: (kt)"),
								container.PlaceWidget(display),
							),
							container.Bottom(
								container.Border(linestyle.Light),
								container.BorderTitle("Turbine Control"),
								container.SplitVertical(
									container.Left(
										container.SplitHorizontal(
											container.Top(
												container.PlaceWidget(rpmSettingMeter),
											),
											container.Bottom(
												container.SplitVertical(
													container.Left(
														container.PlaceWidget(buttonTurbinePlus),
														container.AlignHorizontal(align.HorizontalCenter),
													),
													container.Right(
														container.PlaceWidget(buttonTurbineMinus),
														container.AlignHorizontal(align.HorizontalCenter),
													),
												),
											),
										),
									),
									container.Right(
										container.Border(linestyle.Light),
										container.BorderTitle("rpm"),
										container.PlaceWidget(rpmMeter),
									),
								),
							),
						),
					),
					container.Bottom(
						container.Border(linestyle.Light),
						container.BorderTitle("Wraps lines at rune boundaries"),
						container.PlaceWidget(wrapped),
					),
				),
			),
			container.Right(
				container.SplitHorizontal(
					container.Top(
						container.PlaceWidget(rudderAngleGaugeObj),
					),
					container.Bottom(
						container.Border(linestyle.Light),
						container.BorderTitle("Rolls and scrolls content wrapped at words"),
						container.PlaceWidget(rolled),
					),
				),
			),
		),
	)
	if err != nil {
		panic(err)
	}

	quitter := func(k *terminalapi.Keyboard) {
		if k.Key == 'q' || k.Key == 'Q' {
			cancel()
		}
	}

	if err := termdash.Run(ctx, t, c, termdash.KeyboardSubscriber(quitter), termdash.RedrawInterval(16*time.Millisecond)); err != nil {
		panic(err)
	}

	debugLog("main(): end")
}
