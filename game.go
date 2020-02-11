package indonesia

import (
	"encoding/gob"
	"errors"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"bitbucket.org/SlothNinja/slothninja-games/sn/type"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(setupEntry))
	gob.Register(new(startEntry))
}

func Register(t gType.Type, r *gin.Engine) {
	gob.Register(new(Game))
	game.Register(t, newGamer, PhaseNames, nil)
	AddRoutes(t.Prefix(), r)
}

var ErrMustBeGame = errors.New("Resource must have type *Game.")

const NoPlayerID = game.NoPlayerID

type Game struct {
	*game.Header
	*State
}

type State struct {
	Playerers          game.Playerers
	Log                GameLog
	Era                Era
	AvailableDeeds     Deeds
	Areas              Areas
	CityStones         []int
	Merger             *Merger
	SiapFajiMerger     *SiapFajiMerger
	OverrideDeliveries int
	Version            int
	*TempData
}

// Non-persistent values
// They are memcached but ignored by datastore
type TempData struct {
	SelectedSlot             int
	SelectedAreaID           AreaID
	SelectedArea2ID          AreaID
	SelectedGoodsAreaID      AreaID
	SelectedShippingProvince Province
	OldSelectedAreaID        AreaID
	SelectedShipperIndex     int
	SelectedShipper2Index    int
	SelectedCardIndex        int
	SelectedPlayerID         int
	SelectedTechnology       Technology
	SelectedDeedIndex        int
	ShippingCompanyOwnerID   int
	ShippingCompanySlot      int
	ShipsUsed                int
	Expansions               int
	RequiredExpansions       int
	RequiredDeliveries       int
	ProposedPath             flowMatrix
	CustomPath               flowMatrix
	ShipperIncomeMap         ShipperIncomeMap
	Admin                    bool
	AdminAction              string
}

type Era int

const (
	NoEra Era = iota
	EraA
	EraB
	EraC
)

func (e Era) String() string {
	switch e {
	case EraA:
		return "a"
	case EraB:
		return "b"
	case EraC:
		return "c"
	default:
		return ""
	}
}

type ShipType int
type ShipTypes []ShipType

const (
	NoShipType ShipType = iota
	RedShipA
	YellowShipA
	BlueShipA
	RedShipB
	YellowShipB
	BlueShipB
)

var validShipTypes = ShipTypes{
	RedShipA,
	YellowShipA,
	BlueShipA,
	RedShipB,
	YellowShipB,
	BlueShipB,
}

var shipTypeStringMap = map[ShipType]string{
	NoShipType:  "None",
	RedShipA:    "Red Ship A",
	YellowShipA: "Yellow Ship A",
	BlueShipA:   "Blue Ship A",
	RedShipB:    "Red Ship B",
	YellowShipB: "Yellow Ship B",
	BlueShipB:   "Blue Ship B",
}

func (s ShipType) String() string {
	return shipTypeStringMap[s]
}

func (s ShipType) IDString() string {
	return restful.IDString(s.String())
}

func (g *Game) GetPlayerers() game.Playerers {
	return g.Playerers
}

func (g *Game) Players() (players Players) {
	ps := g.GetPlayerers()
	length := len(ps)
	if length > 0 {
		players = make(Players, length)
		for i, p := range ps {
			players[i] = p.(*Player)
		}
	}
	return
}

func (g *Game) setPlayers(players Players) {
	length := len(players)
	if length > 0 {
		ps := make(game.Playerers, length)
		for i, p := range players {
			ps[i] = p
		}
		g.Playerers = ps
	}
}

type Games []*Game

func (g *Game) Start(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Status = game.Running
	g.Version = 2
	g.setupPhase(ctx)
	return
}

func (g *Game) addNewPlayers() {
	for _, u := range g.Users {
		g.addNewPlayer(u)
	}
}

func (g *Game) setupPhase(ctx context.Context) {
	g.Turn = 0
	g.Phase = Setup
	g.CityStones = []int{12, 8, 3}
	g.addNewPlayers()
	g.RandomTurnOrder()
	g.dealCityCards()
	g.createAreas()
	for _, p := range g.Players() {
		g.newSetupEntryFor(p)
	}
	g.beginningOfPhaseReset()
	g.start(ctx)
	return
}

func (g *Game) getAvailableShipType() ShipType {
	for _, shipType := range validShipTypes {
		found := false
		for _, shippingCompany := range g.ShippingCompanies() {
			if shippingCompany.ShipType == shipType {
				found = true
				break
			}
		}
		if !found {
			return shipType
		}
	}
	return NoShipType
}

type setupEntry struct {
	*Entry
}

func (g *Game) newSetupEntryFor(p *Player) (e *setupEntry) {
	e = new(setupEntry)
	e.Entry = g.newEntryFor(p)
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *setupEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("%s received 100 rupiah and 3 city cards.", g.NameByPID(e.PlayerID))
}

func (g *Game) start(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = StartGame
	g.newStartEntry()
	g.startNewEra(ctx)
	return
}

type startEntry struct {
	*Entry
}

func (g *Game) newStartEntry() *startEntry {
	e := new(startEntry)
	e.Entry = g.newEntry()
	g.Log = append(g.Log, e)
	return e
}

func (e *startEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	names := make([]string, g.NumPlayers)
	for i, p := range g.Players() {
		names[i] = g.NameFor(p)
	}
	return restful.HTML("Good luck %s.  Have fun.", restful.ToSentence(names))
}

func (g *Game) setCurrentPlayers(players ...*Player) {
	var playerers game.Playerers

	switch length := len(players); {
	case length == 0:
		playerers = nil
	case length == 1:
		playerers = game.Playerers{players[0]}
	default:
		playerers = make(game.Playerers, length)
		for i, player := range players {
			playerers[i] = player
		}
	}
	g.SetCurrentPlayerers(playerers...)
}

func (g *Game) PlayerByID(id int) (player *Player) {
	if p := g.PlayererByID(id); p != nil {
		player = p.(*Player)
	}
	return
}

func (g *Game) PlayerBySID(sid string) (player *Player) {
	if p := g.Header.PlayerBySID(sid); p != nil {
		player = p.(*Player)
	}
	return
}

func (g *Game) PlayerByUserID(id int64) (player *Player) {
	if p := g.PlayererByUserID(id); p != nil {
		player = p.(*Player)
	}
	return
}

func (g *Game) PlayerByIndex(index int) (player *Player) {
	if p := g.PlayererByIndex(index); p != nil {
		player = p.(*Player)
	}
	return
}

func (g *Game) undoAction(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if tmpl, err = g.undoRedoReset(ctx, "%s undid action."); err == nil {
		act = game.Undo
	} else {
		act = game.None
	}
	return
}

func (g Game) redoAction(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if tmpl, err = g.undoRedoReset(ctx, "%s redid action."); err == nil {
		act = game.Redo
	} else {
		act = game.None
	}
	return
}

func (g *Game) resetTurn(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if tmpl, err = g.undoRedoReset(ctx, "%s reset turn."); err == nil {
		act = game.Reset
	} else {
		act = game.None
	}
	return
}

func (g *Game) undoRedoReset(ctx context.Context, fmt string) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	cp := g.CurrentPlayer()
	if !g.CUserIsCPlayerOrAdmin(ctx) {
		err = sn.NewVError("Only the current player may perform this action.")
	}

	restful.AddNoticef(ctx, fmt, g.NameFor(cp))
	return
}

func (g *Game) CurrentPlayer() (player *Player) {
	if p := g.CurrentPlayerer(); p != nil {
		player = p.(*Player)
	}
	return
}

type sslice []string

func (ss sslice) include(s string) bool {
	for _, str := range ss {
		if str == s {
			return true
		}
	}
	return false
}

//var headerValues = sslice{
//	"Header.Title",
//	"Header.Turn",
//	"Header.Phase",
//	"Header.SubPhase",
//	"Header.Round",
//	"Header.Password",
//	"Header.CPUserIndices",
//	"Header.UserIDS",
//	"Header.WinnerIDS",
//	"Header.Status",
//	"State.OverrideDeliveries",
//	"State.CityStones",
//	"State.SiapFajiMerger.OwnerID",
//	"State.SiapFajiMerger.OwnerSlot",
//	"State.SiapFajiMerger.Production",
//}

func (g *Game) adminHeader(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	h := game.NewHeader(ctx, nil)
	if err = restful.BindWith(ctx, h, binding.FormPost); err != nil {
		act = game.None
		return
	}

	g.Title = h.Title
	g.Turn = h.Turn
	g.Phase = h.Phase
	g.SubPhase = h.SubPhase
	g.Round = h.Round
	g.NumPlayers = h.NumPlayers
	g.Password = h.Password
	g.CreatorID = h.CreatorID
	g.UserIDS = h.UserIDS
	g.OrderIDS = h.OrderIDS
	g.CPUserIndices = h.CPUserIndices
	g.WinnerIDS = h.WinnerIDS
	g.Status = h.Status
	act = game.Save
	return
}

func (g *Game) adminCities(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	form := new(struct {
		CityStones []int `form:"city-stones"`
	})

	if err = restful.BindWith(ctx, form, binding.FormPost); err != nil {
		act = game.None
		return
	}

	for i, s := range form.CityStones {
		g.CityStones[i] = s
	}

	act = game.Save
	return
}

//func adminHeader(g *Game, form url.Values) (string, game.ActionType, error) {
//	if err := g.adminUpdateHeader(headerValues); err != nil {
//		return "indonesia/flash_notice", game.None, err
//	}
//
//	return "", game.Save, nil
//}
//
//func (g *Game) adminUpdateHeader(ss sslice) (err error) {
//	if err = g.validateAdminAction(ctx); err != nil {
//		return err
//	}
//
//	//g.debugf("Values: %#v", values)
//	mergerRemove, siapFajiMergerRemove := false, false
//	//	addDeedIndex, removeDeedIndex := -1, -1
//	for key := range values {
//		if key == "MergerRemove" {
//			if value := values.Get(key); value == "true" {
//				mergerRemove = true
//			}
//		}
//		if key == "MergerRemove" {
//			if value := values.Get(key); value == "true" {
//				siapFajiMergerRemove = true
//			}
//		}
//		if key == "AddAvailableDeed" {
//			if k := values.Get(key); k != "none" {
//				if d := g.Deeds().get(k); d != nil {
//					g.AvailableDeeds = append(g.AvailableDeeds, d)
//				}
//			}
//		}
//		if key == "RemoveAvailableDeed" {
//			if k := values.Get(key); k != "none" {
//				if d := g.AvailableDeeds.get(k); d != nil {
//					g.AvailableDeeds = g.AvailableDeeds.remove(d)
//				}
//			}
//		}
//		if !ss.include(key) {
//			delete(values, key)
//		}
//	}
//
//	schema.RegisterConverter(game.Phase(0), convertPhase)
//	schema.RegisterConverter(game.SubPhase(0), convertSubPhase)
//	schema.RegisterConverter(game.Status(0), convertStatus)
//	//	game.RegisterDBIDConverter()
//	if err := schema.Decode(g, values); err != nil {
//		return err
//	}
//	if mergerRemove {
//		g.Merger = nil
//	}
//	if siapFajiMergerRemove {
//		g.SiapFajiMerger = nil
//	}
//	//	if addDeedIndex != -1 {
//	//		g.AvailableDeeds = append(g.AvailableDeeds, g.Deeds()[addDeedIndex])
//	//	}
//	//	if removeDeedIndex != -1 {
//	//		g.AvailableDeeds = g.AvailableDeeds.removeAt(removeDeedIndex)
//	//	}
//	return nil
//}
//
//func convertPhase(value string) reflect.Value {
//	if v, err := strconv.ParseInt(value, 10, 0); err == nil {
//		return reflect.ValueOf(game.Phase(v))
//	}
//	return reflect.Value{}
//}
//
//func convertSubPhase(value string) reflect.Value {
//	if v, err := strconv.ParseInt(value, 10, 0); err == nil {
//		return reflect.ValueOf(game.SubPhase(v))
//	}
//	return reflect.Value{}
//}
//
//func convertStatus(value string) reflect.Value {
//	if v, err := strconv.ParseInt(value, 10, 0); err == nil {
//		return reflect.ValueOf(game.Status(v))
//	}
//	return reflect.Value{}
//}

func (g *Game) SelectedPlayer() *Player {
	return g.PlayerByID(g.SelectedPlayerID)
}

func (g *Game) setSelectedPlayer(p *Player) {
	if p != nil {
		g.SelectedPlayerID = p.ID()
	} else {
		g.SelectedPlayerID = NoPlayerID
	}
}

func min(ints ...int) int {
	if len(ints) <= 0 {
		return 0
	}

	min := ints[0]
	for _, i := range ints {
		if i < min {
			min = i
		}
	}
	return min
}

func max(ints ...int) int {
	if len(ints) <= 0 {
		return 0
	}

	max := ints[0]
	for _, i := range ints {
		if i > max {
			max = i
		}
	}
	return max
}
