package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/inkyblackness/imgui-go/v2"

	"github.com/fourst4r/levedit/pr2hub"

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

var (
	blocksImage *ebiten.Image
	blockImgs   map[int]*ebiten.Image
)

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
	blockImgs = make(map[int]*ebiten.Image)
	for i := 0; i < len(blocks); i++ {
		sx := (i % tileXNum) * tileSize
		sy := (i / tileXNum) * tileSize
		subimg := blocksImage.SubImage(image.Rect(sx, sy, sx+tileSize, sy+tileSize)).(*ebiten.Image)
		blockImgs[i] = subimg
	}
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
	loginremember        bool
	loginstatus          string
	loginresp            *pr2hub.LoginResponse
	// load
	levelsgetresp  pr2hub.LevelsGetResponse
	levelsgotten   bool
	levelsselected int32
	levelsnames    []string
	// download level
	dldone  bool
	dllevel string
	// goto
	gotoX, gotoY int32
	// save
	saveresp string
	// delete
	deleteresp pr2hub.DeleteLevelResponse

	req    *pr2hub.Req
	config *Config
}

const (
	camSpeed  float64 = 5
	zoomSpeed         = 1.2
	zoomMin           = 0.01
	zoomMax           = 2.5
)

func (e *Editor) Update(screen *ebiten.Image) error {
	e.mgr.Update(1.0/60.0, float32(e.w), float32(e.h))
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		// g.mgr.ClipMask = !g.mgr.ClipMask
	}

	speed := camSpeed
	if ebiten.IsKeyPressed(ebiten.KeyShift) {
		speed *= 4
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

		lmb := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
		rmb := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
		if lmb || rmb {
			g := e.centerCam(screen)
			if g.IsInvertible() {
				g.Invert()
				mx, my := ebiten.CursorPosition()
				worldx, worldy := g.Apply(float64(mx), float64(my))
				blockx, blocky := int(math.Floor(worldx/tileSize)), int(math.Floor(worldy/tileSize))
				gridx, gridy := blockx*tileSize, blocky*tileSize

				if lmb {
					if _, ok := e.Course.Blocks.Peek(gridx, gridy); !ok {
						e.Course.Blocks.Push(gridx, gridy, int(e.block))
					}
				} else if rmb {
					e.Course.Blocks.Pop(gridx, gridy)
				}
			} else {
				log.Println("cam is not invertible?", g)
			}
		}
	}

	return nil
}

func (e *Editor) centerCam(screen *ebiten.Image) ebiten.GeoM {
	bounds := screen.Bounds()
	var centerX, centerY = float64(bounds.Dx()) / 2, float64(bounds.Dy()) / 2

	centerCam := ebiten.GeoM{}
	centerCam.Concat(e.cam)
	centerCam.Translate(centerX, centerY) // center on screen
	// centerCam.Translate(tileSize/2, tileSize/2)         // center on block
	scaleAround(&centerCam, -centerX, -centerY, e.zoom) // zoom around center
	return centerCam
}

const (
	tileSize = 30
	tileXNum = 10
)

func (e *Editor) Draw(screen *ebiten.Image) {
	e.mgr.BeginFrame()
	e.drawUI()

	screen.Fill(e.Course.BackgroundColor)

	// bounds := screen.Bounds()
	// var centerX, centerY = float64(bounds.Dx()) / 2, float64(bounds.Dy()) / 2

	centerCam := e.centerCam(screen) //ebiten.GeoM{}
	// centerCam.Concat(e.cam)
	// centerCam.Translate(centerX, centerY)               // center on screen
	// centerCam.Translate(-tileSize/2, -tileSize/2)       // center on block
	// scaleAround(&centerCam, -centerX, -centerY, e.zoom) // zoom around center

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
			imgui.OpenPopup(PopupGoto)
		}
	}
	e.gotoPopup()

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

				// imgui.ColumnsV(3, "Blocks", false)
				for i := 0; i < len(blocks); i++ {
					if i%8 != 0 {
						imgui.SameLine()
					}
					id := 100 + i
					if imgui.ImageButton(imgui.TextureID(id), imgui.Vec2{tileSize, tileSize}) {
						e.block = course.Block(id - 100)
						log.Println("pressed", id)
					}
					// if id%10 == 0 {
					// 	imgui.NextColumn()
					// }
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
				var bgc [3]float32 = coltof3(e.Course.BackgroundColor)
				if imgui.ColorEdit3V("Background Color", &bgc, imgui.ColorEditFlagsHEX) {
					e.Course.BackgroundColor = f3tocol(bgc)
				}
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
			imgui.OpenPopup(PopupLogin)
		}
		e.loginPopup()

		imgui.SameLine()

		if imgui.Button("Save") {
			imgui.OpenPopup(PopupSave)
		}
		e.savePopup()

		imgui.SameLine()

		if imgui.Button("Load File") {

		}

		imgui.SameLine()

		if imgui.Button("Load") {
			e.req = pr2hub.LevelsGet()
			e.levelsnames = []string{"Loading..."}
			e.levelsgotten = false
			imgui.OpenPopup(PopupLoad)
		}
		e.loadPopup()

		imgui.SameLine()

		if imgui.Button("New") {
			e.loadCourse(course.Default())
		}

		imgui.SameLine()

		preview := "guest"
		if e.config.SelectedAcc < len(e.config.Accs) {
			preview = e.config.Accs[e.config.SelectedAcc].User
		}
		if imgui.BeginCombo("Account", preview) {
			for i, acc := range e.config.Accs {
				if imgui.Selectable(acc.User) {
					e.config.SelectedAcc = i
					e.setAcc(e.config.Accs[i])
					if err := e.config.Save(); err != nil {
						log.Println(err)
					}
				}
			}
			imgui.EndCombo()
		}
	}

	imgui.End()
}

const (
	PopupSave         = "PopupSave"
	PopupSaveProgress = "PopupSaveProgress"
	PopupSaveResponse = "PopupSaveResponse"
)

func (e *Editor) savePopup() {
	// PopupSave
	imgui.SetNextWindowSize(imgui.Vec2{X: 300, Y: 0})
	if imgui.BeginPopupModalV(PopupSave, nil, imgui.WindowFlagsNone) {
		imgui.InputText("Title", &e.Course.Title)
		imgui.InputTextMultiline("Note", &e.Course.Note)
		imgui.Checkbox("Publish", &e.Course.Live)
		if imgui.Button("Save") {
			acc := e.config.Accs[e.config.SelectedAcc]
			// log.Println("save", acc.User, acc.Token)
			var err error
			e.req, err = pr2hub.UploadLevel(e.Course.String(acc.User, acc.Token))
			if err != nil {
				log.Println(err)
			}
			imgui.CloseCurrentPopup()
			defer imgui.OpenPopup(PopupSaveProgress)
		}
		imgui.SameLine()
		if imgui.Button("Cancel") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
	// PopupSaveProgress
	if imgui.BeginPopupModalV(PopupSaveProgress, nil, imgui.WindowFlagsAlwaysAutoResize) {
		if e.req.Done(&e.saveresp) {
			if err := e.req.Err(); err != nil {
				log.Println(err)
			} else {
				log.Println(e.saveresp)
			}
			imgui.CloseCurrentPopup()
			defer imgui.OpenPopup(PopupSaveResponse)
		}

		imgui.Text(fmt.Sprintf("Saving the level... %c", spinner()))
		imgui.EndPopup()
	}
	// PopupSaveResponse
	if imgui.BeginPopupModalV(PopupSaveResponse, nil, imgui.WindowFlagsAlwaysAutoResize) {
		imgui.Text(e.saveresp)
		if imgui.Button("OK") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
}

const (
	PopupLoad         = "PopupLoad"
	PopupLoadProgress = "PopupLoadProgress"
	PopupLoadFailure  = "PopupLoadFailure"
)

func (e *Editor) loadPopup() {
	// PopupLoad
	imgui.SetNextWindowSize(imgui.Vec2{X: 400, Y: 0})
	if imgui.BeginPopupModalV(PopupLoad, nil, imgui.WindowFlagsNone) {
		if !e.levelsgotten {
			e.levelsnames[0] = fmt.Sprintf("Loading... %c", spinner())
			if e.req.Done(&e.levelsgetresp) {
				// we got the levels, now stop checking req.Done()
				e.levelsgotten = true
				if err := e.req.Err(); err != nil {
					log.Println(err)
				} else {
					r := e.levelsgetresp
					if r.Success {
						e.levelsnames = make([]string, len(r.Levels))
						for i, level := range r.Levels {
							e.levelsnames[i] = level.Title
						}
					} else {
						msg := fmt.Sprintf("pr2hub: %s", r.Error)
						e.levelsnames = []string{msg}
					}
				}
			}
		}

		imgui.ColumnsV(2, "col", true)

		imgui.PushItemWidth(-1) // don't show label
		imgui.ListBoxV("", &e.levelsselected, e.levelsnames, 20)
		imgui.PopItemWidth()

		imgui.NextColumn()
		if e.levelsgotten && int(e.levelsselected) < len(e.levelsgetresp.Levels) {
			imgui.PushTextWrapPos()
			imgui.Text(e.levelsgetresp.Levels[e.levelsselected].Note)
			imgui.PopTextWrapPos()
		}
		imgui.Columns()
		imgui.Separator()
		if imgui.Button("Load##2") && int(e.levelsselected) < len(e.levelsgetresp.Levels) {
			level := e.levelsgetresp.Levels[e.levelsselected]
			e.req = pr2hub.Level(level.LevelID, level.Version)
			imgui.CloseCurrentPopup()
			defer imgui.OpenPopup(PopupLoadProgress)
		}
		imgui.SameLine()

		if imgui.Button("Delete") && int(e.levelsselected) < len(e.levelsgetresp.Levels) {
			level := e.levelsgetresp.Levels[e.levelsselected]
			token := e.config.Accs[e.config.SelectedAcc].Token
			var err error
			e.req, err = pr2hub.DeleteLevel(level.LevelID, token)
			if err != nil {
				log.Println(err)
			}
			imgui.OpenPopup(PopupDelete)
		}
		e.deletePopup()

		imgui.SameLine()
		if imgui.Button("Cancel") {
			e.levelsgotten = false
			e.req.Cancel()
			imgui.CloseCurrentPopup()
		}

		imgui.EndPopup()
	}
	// PopupLoadProgress
	if imgui.BeginPopupModalV(PopupLoadProgress, nil, imgui.WindowFlagsAlwaysAutoResize) {
		if e.req.Done(&e.dllevel) {
			e.dldone = true
			if err := e.req.Err(); err != nil {
				log.Println(err)
			} else {
				c, err := course.Parse(e.dllevel)
				if err != nil {
					log.Println(err)
				}
				e.loadCourse(c)
			}
			imgui.CloseCurrentPopup()
		}

		imgui.Text(fmt.Sprintf("Downloading the level... %c", spinner()))
		imgui.EndPopup()
	}
}

const (
	PopupDelete         = "PopupDelete"
	PopupDeleteProgress = "PopupDeleteProgress"
	PopupDeleteResponse = "PopupDeleteResponse"
)

func (e *Editor) deletePopup() {
	// PopupDelete
	if imgui.BeginPopupModalV(PopupDelete, nil, imgui.WindowFlagsNone) {
		level := e.levelsgetresp.Levels[e.levelsselected]
		imgui.Text(fmt.Sprintf("Are you sure you want to delete %q?", level.Title))
		if imgui.Button("Yes") {
			imgui.CloseCurrentPopup()
			defer imgui.OpenPopup(PopupDeleteProgress)
		}
		imgui.SameLine()
		if imgui.Button("No") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
	// PopupDeleteProgress
	if imgui.BeginPopupModalV(PopupDeleteProgress, nil, imgui.WindowFlagsAlwaysAutoResize) {
		if e.req.Done(&e.deleteresp) {
			if err := e.req.Err(); err != nil {
				log.Println(err)
			} else {
				if e.deleteresp.Success {
					// refresh levels list
					e.req = pr2hub.LevelsGet()
					e.levelsnames = []string{"Loading..."}
					e.levelsgotten = false
				} else {
					defer imgui.OpenPopup(PopupDeleteResponse)
				}
			}
			imgui.CloseCurrentPopup()
		}

		imgui.Text(fmt.Sprintf("Deleting the level... %c", spinner()))
		imgui.EndPopup()
	}
	// PopupDeleteResponse
	if imgui.BeginPopupModalV(PopupDeleteResponse, nil, imgui.WindowFlagsAlwaysAutoResize) {
		imgui.Text(e.deleteresp.Error)
		if imgui.Button("OK") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
}

const (
	PopupGoto = "Go to##PopupGoto"
)

func (e *Editor) gotoPopup() {
	if imgui.BeginPopup(PopupGoto) {
		imgui.InputInt("X", &e.gotoX)
		imgui.InputInt("Y", &e.gotoY)
		if imgui.Button("Go To") {
			setPos(&e.cam, float64(e.gotoX), float64(e.gotoY))
		}
		imgui.SameLine()
		if imgui.Button("1") {
			e.gotoBlock(course.BlockPlayer1)
		}
		imgui.SameLine()
		if imgui.Button("2") {
			e.gotoBlock(course.BlockPlayer2)
		}
		imgui.SameLine()
		if imgui.Button("3") {
			e.gotoBlock(course.BlockPlayer3)
		}
		imgui.SameLine()
		if imgui.Button("4") {
			e.gotoBlock(course.BlockPlayer4)
		}
		imgui.EndPopup()
	}
}

const (
	PopupLogin         = "Login##PopupLogin"
	PopupLoginProgress = "Logging in##PopupLoginProgress"
	PopupLoginFailure  = "Login failed##PopupLoginFailure"
)

func (e *Editor) loginPopup() {
	// PopupLogin
	if imgui.BeginPopupModalV(PopupLogin, nil, imgui.WindowFlagsAlwaysAutoResize) {
		imgui.InputText("user", &e.loginuser)
		imgui.InputTextV("pass", &e.loginpass, imgui.InputTextFlagsPassword, nil)
		// imgui.Checkbox("remember?", &e.loginremember)
		if imgui.Button("Log In") {
			var err error
			e.req, err = pr2hub.Login(e.loginuser, e.loginpass, e.loginremember)
			if err == nil {
				imgui.CloseCurrentPopup()
				defer imgui.OpenPopup(PopupLoginProgress)
			} else {
				e.loginstatus = err.Error()
				log.Println(err)
			}
		}
		imgui.SameLine()
		if imgui.Button("Cancel") {
			imgui.CloseCurrentPopup()
		}
		imgui.EndPopup()
	}
	// PopupLoginProgress
	if imgui.BeginPopupModalV(PopupLoginProgress, nil, imgui.WindowFlagsAlwaysAutoResize) {
		var resp pr2hub.LoginResponse
		if e.req.Done(&resp) {
			if err := e.req.Err(); err != nil {
				e.loginstatus = fmt.Sprint("error:", err)
				log.Println(err)
			} else {
				if resp.Success {
					e.loginstatus = fmt.Sprint("Login successful ")
					acc := Acc{e.loginuser, resp.Token}
					e.setAcc(acc)
					e.config.Accs = append(e.config.Accs, acc)
					e.config.SelectedAcc = len(e.config.Accs) - 1
					err = e.config.Save()
					if err != nil {
						log.Println(err)
					}

					// imgui.CloseCurrentPopup()
				} else {
					e.loginstatus = resp.Error
					defer imgui.OpenPopup(PopupLoginFailure)
				}
			}
			imgui.CloseCurrentPopup()
		} else {
			imgui.Text(fmt.Sprintf("Logging in... %c", spinner()))
			if imgui.Button("Cancel") {
				e.req.Cancel()
				imgui.CloseCurrentPopup()
				defer imgui.OpenPopup(PopupLogin)
			}
		}
		imgui.EndPopup()
	}
	// PopupLoginFailure
	if imgui.BeginPopupModalV(PopupLoginFailure, nil, imgui.WindowFlagsAlwaysAutoResize) {
		imgui.Text(e.loginstatus)
		if imgui.Button("OK") {
			imgui.CloseCurrentPopup()
			defer imgui.OpenPopup(PopupLogin)
		}
		imgui.EndPopup()
	}
}

func (e *Editor) setAcc(acc Acc) {
	u, err := url.Parse("https://pr2hub.com")
	if err != nil {
		log.Fatalln(err)
	}
	cookie := &http.Cookie{Name: "token", Value: acc.Token}
	http.DefaultClient.Jar, err = cookiejar.New(nil)
	if err != nil {
		log.Fatalln(err)
	}
	http.DefaultClient.Jar.SetCookies(u, []*http.Cookie{cookie})
	log.Printf("Logged in as %s:%s\n", acc.User, acc.Token)
}

func main() {
	ebiten.SetWindowSize(1280, 960)
	ebiten.SetWindowTitle(fmt.Sprintf("%s v%s", AppName, AppVersion))
	ebiten.SetWindowResizable(true)

	cfg, err := LoadConfig()
	if err != nil {
		log.Println(err)
	}

	e := &Editor{
		mgr:    renderer.New(nil),
		zoom:   1,
		config: cfg,
	}
	if len(cfg.Accs) > 0 {
		if cfg.SelectedAcc > len(cfg.Accs) {
			e.setAcc(cfg.Accs[0])
		} else {
			e.setAcc(cfg.Accs[cfg.SelectedAcc])
		}
	}

	// resp, err := pr2hub.CheckLogin()
	// if err == nil {
	// 	if len(resp.UserName) != 0 {
	// 		log.Println("Logged in as", resp.UserName)
	// 	}
	// 	// cfg.CheckLoginResponse = *resp
	// } else {
	// 	log.Println("CheckLogin failed:", err)
	// }

	e.loadCourse(course.Default())

	for i, subimg := range blockImgs {
		// id := imgui.TextureID((unsafe.Pointer(subimg)))
		e.mgr.Cache.SetTexture(imgui.TextureID(100+i), subimg)
	}

	if err := ebiten.RunGame(e); err != nil {
		panic(err)
	}
}

func (e *Editor) loadCourse(c *course.Course) {
	e.Course = c
	e.gotoBlock(course.BlockPlayer1)
}

func (e *Editor) gotoBlock(b int) {
	for xy, stack := range e.Course.Blocks {
		for _, block := range stack {
			if b == block.(int) || b == block.(int)-100 {
				x, y := xytof(xy)
				setPos(&e.cam, -x, -y)
				return
			}
		}
	}

}

func (e *Editor) screenToWorld(screen *ebiten.Image, x, y float64) (float64, float64) {
	g := e.centerCam(screen)
	if g.IsInvertible() {
		g.Invert()
		return g.Apply(x, y)
	}
	log.Println("cam is not invertible?", g)
	return math.NaN(), math.NaN()
}

func setPos(g *ebiten.GeoM, x, y float64) {
	g.SetElement(0, 2, x)
	g.SetElement(1, 2, y)
}

func scaleAround(g *ebiten.GeoM, x, y, scale float64) {
	g.Translate(x, y)
	g.Scale(scale, scale)
	g.Translate(-x, -y)
}

func spinner() byte {
	return "|/-\\"[int(imgui.Time()/0.05)&3]
}

func xytof(xy course.XY) (x, y float64) {
	return float64(xy.X), float64(xy.Y)
}
