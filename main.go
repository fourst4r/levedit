package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"

	"github.com/inkyblackness/imgui-go/v2"

	"levedit/pr2hub"

	"github.com/fourst4r/course"
	"github.com/gabstv/ebiten-imgui/renderer"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	"github.com/hajimehoshi/ebiten/inpututil"
	"golang.org/x/image/colornames"
)

var (
	blocks = []string{
		// "Basic1", "Basic2", "Basic3", "Basic4", "Brick", "Finish", "Ice", "Item",
		// "Item+", "Left", "Right", "Up", "Down", "Mine", "Crumble", "Vanish", "Move",
		// "Water", "Rotate Right", "Rotate Left", "Push", "Happy", "Sad", "Net", "Heart",
		// "Time", "Egg",
		"Basic1", "Basic2", "Basic3", "Basic4", "Brick", "Down", "Up", "Left", "Right", "Mine",
		"Item", "Player1", "Player2", "Player3", "Player4", "Ice", "Finish", "Crumble", "Vanish", "Move",
		"Water", "Rotate Right", "Rotate Left", "Push", "Net", "Item+", "Happy", "Sad", "Heart", "Time",
		"Egg",
	}
	songs = []string{
		"None", "Random", "Orbital Trance - Space Planet",
	}
)

var blocksImage *ebiten.Image

func init() {
	b, err := ioutil.ReadFile("assets/pr2-blocks.png")
	if err != nil {
		log.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		log.Fatal(err)
	}
	blocksImage, _ = ebiten.NewImageFromImage(img, ebiten.FilterDefault)
}

type Editor struct {
	mgr    *renderer.Manager
	Course *course.Course
	cam    ebiten.GeoM
	zoom   float64
	w, h   int

	art00, art0, blocks, art1, art2, art3, bg, settings bool

	// blocks
	block course.Block
	// bg
	backgroundColor [3]float32
	// settings
	song         int
	items        [9]bool
	minRank      int
	gravity      float64
	time         int
	mode         string
	cowboyChance int
	pass         string
	// login
	loginuser, loginpass string
	loginstatus          string
	loginresp            *pr2hub.LoginResponse
	// goto
	gotoX, gotoY int32

	config struct {
		pr2hub.CheckLoginResponse
	}
}

const (
	camSpeed  float64 = 5
	zoomSpeed         = 1.2
	zoomMin           = 0.1
	zoomMax           = 2.0
)

func (e *Editor) Update(screen *ebiten.Image) error {
	e.mgr.Update(1.0/60.0, float32(e.w), float32(e.h))
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		// g.mgr.ClipMask = !g.mgr.ClipMask
	}

	speed := camSpeed
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		speed *= 2
	}

	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		e.cam.Translate(speed, 0)
	} else if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		e.cam.Translate(-speed, 0)
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		e.cam.Translate(0, speed)
	} else if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		e.cam.Translate(0, -speed)
	}
	if !imgui.IsWindowHoveredV(imgui.HoveredFlagsAnyWindow) {
		_, yoff := ebiten.Wheel()
		e.zoom *= math.Pow(zoomSpeed, yoff)
		e.zoom = clamp(e.zoom, zoomMin, zoomMax)
	}
	return nil
}

func (e *Editor) Draw(screen *ebiten.Image) {
	e.mgr.BeginFrame()
	e.drawUI()

	screen.Fill(color.RGBA{
		uint8(e.backgroundColor[0] * 0xff),
		uint8(e.backgroundColor[1] * 0xff),
		uint8(e.backgroundColor[2] * 0xff),
		0xff,
	})

	const tileXNum = 10
	const tileSize = 30
	bounds := screen.Bounds()
	var centerX, centerY = float64(bounds.Dx()) / 2, float64(bounds.Dy()) / 2

	centerCam := ebiten.GeoM{}
	centerCam.Concat(e.cam)
	centerCam.Translate(centerX, centerY)               // center on screen
	centerCam.Translate(-tileSize/2, -tileSize/2)       // center on block
	scaleAround(&centerCam, -centerX, -centerY, e.zoom) // zoom around center

	axisX1, axisY1 := centerCam.Apply(-9999999, 0)
	axisX2, axisY2 := centerCam.Apply(9999999, 0)
	ebitenutil.DrawLine(screen, axisX1, axisY1, axisX2, axisY2, colornames.Red)

	axisX1, axisY1 = centerCam.Apply(0, -9999999)
	axisX2, axisY2 = centerCam.Apply(0, 9999999)
	ebitenutil.DrawLine(screen, axisX1, axisY1, axisX2, axisY2, colornames.Limegreen)

	for xy, stack := range e.Course.Blocks {
		for _, block := range stack {
			bID := block.(int)
			if bID > 99 {
				bID -= 100
			}

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(xytof(xy))
			op.GeoM.Concat(centerCam)

			sx := (bID % tileXNum) * tileSize
			sy := (bID / tileXNum) * tileSize
			screen.DrawImage(blocksImage.SubImage(image.Rect(sx, sy, sx+tileSize, sy+tileSize)).(*ebiten.Image), op)
		}
	}

	// draw tool cursor
	op := &ebiten.DrawImageOptions{}
	op.ColorM.Translate(0, 0, 0, -.5)
	op.GeoM.Translate(-tileSize/2, -tileSize/2) // center to cursor
	op.GeoM.Scale(e.zoom, e.zoom)
	mx, my := ebiten.CursorPosition()
	op.GeoM.Translate(float64(mx), float64(my))
	sx := (int(e.block) % tileXNum) * tileSize
	sy := (int(e.block) / tileXNum) * tileSize
	screen.DrawImage(blocksImage.SubImage(image.Rect(sx, sy, sx+tileSize, sy+tileSize)).(*ebiten.Image), op)

	e.mgr.EndFrame(screen)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("cam=%.1f,%.1f zoom=%.1f\ntps=%.2f", e.cam.Element(0, 2), e.cam.Element(1, 2), e.zoom, ebiten.CurrentTPS()))
}

func (e *Editor) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	// if g.retina {
	// 	m := ebiten.DeviceScaleFactor()
	// 	g.w = int(float64(outsideWidth) * m)
	// 	g.h = int(float64(outsideHeight) * m)
	// } else {
	e.w = outsideWidth
	e.h = outsideHeight
	// }
	return e.w, e.h
	// return 320, 240
}

func (e *Editor) drawUI() {
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		if inpututil.IsKeyJustPressed(ebiten.KeyG) {
			imgui.OpenPopup("Go To")
		}
	}

	if imgui.BeginPopup("Go To") {
		imgui.InputInt("X", &e.gotoX)
		imgui.InputInt("Y", &e.gotoY)
		if imgui.Button("Go To") {
			e.cam.SetElement(0, 2, float64(e.gotoX))
			e.cam.SetElement(1, 2, float64(e.gotoY))
		}
		imgui.EndPopup()
	}

	if imgui.Begin("Toolbar") {
		if imgui.BeginTabBar("Tools") {
			if imgui.BeginTabItem("Art00") {
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Art0") {
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Blocks") {
				if imgui.BeginCombo("Block", blocks[e.block]) {
					for id, name := range blocks {
						if imgui.Selectable(name) {
							e.block = course.Block(id)
						}
					}

					imgui.EndCombo()
				}
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Art1") {
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Art2") {
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Art3") {
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("BG") {
				imgui.ColorEdit3("Background Color", &e.backgroundColor)
				imgui.EndTabItem()
			}
			if imgui.BeginTabItem("Settings") {
				if imgui.BeginCombo("Music", songs[e.song]) {
					for id, name := range songs {
						if imgui.Selectable(name) {
							e.song = id
						}
					}
					imgui.EndCombo()
				}
				if imgui.CollapsingHeader("Items") {
					switch {
					case
						imgui.Checkbox("Laser Gun", &e.items[0]),
						imgui.Checkbox("Speed Burst", &e.items[1]),
						imgui.Checkbox("Jet Pack", &e.items[2]),
						imgui.Checkbox("Super Jump", &e.items[3]),
						imgui.Checkbox("Lightning", &e.items[4]),
						imgui.Checkbox("Sword", &e.items[5]),
						imgui.Checkbox("Teleport", &e.items[6]),
						imgui.Checkbox("Mine", &e.items[7]),
						imgui.Checkbox("Ice Wave", &e.items[8]):
					}
				}
				imgui.EndTabItem()
			}
			imgui.EndTabBar()
		}
	}
	imgui.End()

	flags := imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoTitleBar |
		imgui.WindowFlagsAlwaysAutoResize
	if imgui.BeginV("Functionbar", nil, flags) {
		if imgui.Button("Login") {
			imgui.OpenPopup("Login")
		}
		if imgui.BeginPopupModalV("Login", nil, imgui.WindowFlagsAlwaysAutoResize) {
			imgui.InputText("user", &e.loginuser)
			imgui.InputTextV("pass", &e.loginpass, imgui.InputTextFlagsPassword, nil)
			if len(e.loginstatus) != 0 {
				imgui.PushTextWrapPos()
				imgui.Text(e.loginstatus)
				imgui.PopTextWrapPos()
			}
			if imgui.Button("Log In") {
				resp, err := pr2hub.Login(e.loginuser, e.loginpass)
				if err != nil {
					e.loginstatus = fmt.Sprint("error:", err)
				}
				e.loginresp = resp
				if resp.Success {
					e.loginstatus = fmt.Sprint("Login successful ")
				} else {
					e.loginstatus = resp.Error
				}
				// fmt.Printf("user=%s pass=%s token=%s", e.loginuser, e.loginpass, resp.Token)
			}
			imgui.SameLine()
			if imgui.Button("Cancel") {
				imgui.CloseCurrentPopup()
			}
			imgui.EndPopup()
		}
		imgui.SameLine()
		if imgui.Button("Save") {

		}
		imgui.SameLine()
		if imgui.Button("New") {

		}
	}
	imgui.End()
}

func xytof(xy course.XY) (x, y float64) {
	return float64(xy.X), float64(xy.Y)
}

func (e *Editor) loadConfig() {
	// ioutil.ReadAll()
}

func (e *Editor) saveConfig() {

}

func main() {
	ebiten.SetWindowSize(640, 480)
	ebiten.SetWindowTitle("levedit")
	ebiten.SetWindowResizable(true)

	e := &Editor{
		Course: course.Default(),
		mgr:    renderer.New(nil),
		zoom:   1,
	}
outer:
	for xy, stack := range e.Course.Blocks {
		for _, block := range stack {
			if 111 == block.(int) {
				x, y := xytof(xy)
				e.cam.Translate(-x, -y)
				break outer
			}
		}
	}
	e.Course.Blocks.Push(0, 0, 0)

	if err := ebiten.RunGame(e); err != nil {
		panic(err)
	}
}

func scaleAround(g *ebiten.GeoM, x, y, scale float64) {
	g.Translate(x, y)
	g.Scale(scale, scale)
	g.Translate(-x, -y)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
