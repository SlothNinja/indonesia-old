package indonesia

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/codec"
	"bitbucket.org/SlothNinja/slothninja-games/sn/color"
	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/mlog"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"bitbucket.org/SlothNinja/slothninja-games/sn/type"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user/stats"
	"github.com/gin-gonic/gin"
	"go.chromium.org/gae/service/datastore"
	"go.chromium.org/gae/service/info"
	"go.chromium.org/gae/service/memcache"
	"golang.org/x/net/context"
)

const (
	gameKey   = "Game"
	homePath  = "/"
	jsonKey   = "JSON"
	statusKey = "Status"
	hParam    = "hid"
)

func gameFrom(ctx context.Context) (g *Game) {
	g, _ = ctx.Value(gameKey).(*Game)
	return
}

func withGame(c *gin.Context, g *Game) *gin.Context {
	c.Set(gameKey, g)
	return c
}

func jsonFrom(ctx context.Context) (g *Game) {
	g, _ = ctx.Value(jsonKey).(*Game)
	return
}

func withJSON(c *gin.Context, g *Game) *gin.Context {
	c.Set(jsonKey, g)
	return c
}

//type Action func(*Game, url.Values) (string, game.ActionType, error)
//
//var actionMap = map[string]Action{
//	"select-area":              selectArea,
//	"select-hull-player":       selectHullPlayer,
//	"turn-order-bid":           placeTurnOrderBid,
//	"stop-expanding":           stopExpanding,
//	"accept-proposed-flow":     acceptProposedFlow,
//	"city-growth":              cityGrowth,
//	"pass":                     pass,
//	"merger-bid":               mergerBid,
//	"undo":                     undoAction,
//	"redo":                     redoAction,
//	"reset":                    resetTurn,
//	"finish":                   finishTurn,
//	"admin-header":             adminHeader,
//	"admin-area":               adminArea,
//	"admin-patch":              adminPatch,
//	"admin-player":             adminPlayer,
//	"admin-company":            adminCompany,
//	"admin-player-new-company": adminPlayerNewCompany,
//}

func (g *Game) Update(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch a := restful.GinFrom(ctx).PostForm("action"); a {
	case "select-area":
		tmpl, act, err = g.selectArea(ctx)
	case "select-hull-player":
		tmpl, act, err = g.selectHullPlayer(ctx)
	case "turn-order-bid":
		tmpl, act, err = g.placeTurnOrderBid(ctx)
	case "stop-expanding":
		tmpl, act, err = g.stopExpanding(ctx)
	case "accept-proposed-flow":
		tmpl, act, err = g.acceptProposedFlow(ctx)
	case "city-growth":
		tmpl, act, err = g.cityGrowth(ctx)
	case "pass":
		tmpl, act, err = g.pass(ctx)
	case "merger-bid":
		tmpl, act, err = g.mergerBid(ctx)
	case "undo":
		tmpl, act, err = g.undoAction(ctx)
	case "redo":
		tmpl, act, err = g.redoAction(ctx)
	case "reset":
		tmpl, act, err = g.resetTurn(ctx)
	//	case "finish":
	//		tmpl, act, err = g.finishTurn(ctx)
	case "admin-header":
		tmpl, act, err = g.adminHeader(ctx)
	case "admin-cities":
		tmpl, act, err = g.adminCities(ctx)
		//	"admin-area":               adminArea,
		//	"admin-patch":              adminPatch,
		//	"admin-player":             adminPlayer,
		//	"admin-company":            adminCompany,
		//	"admin-player-new-company": adminPlayerNewCompany,
	default:
		tmpl, act, err = "indonesia/flash_notice", game.None, sn.NewVError("%v is not a valid action.", a)
	}
	return
}

func Show(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")

		g := gameFrom(ctx)
		cu := user.CurrentFrom(ctx)
		c.HTML(http.StatusOK, prefix+"/show", gin.H{
			"Context":    ctx,
			"VersionID":  info.VersionID(ctx),
			"CUser":      cu,
			"Game":       g,
			"IsAdmin":    user.IsAdmin(ctx),
			"Admin":      game.AdminFrom(ctx),
			"MessageLog": mlog.From(ctx),
			"ColorMap":   color.MapFrom(ctx),
			"Notices":    restful.NoticesFrom(ctx),
			"Errors":     restful.ErrorsFrom(ctx),
		})
	}
}

func Update(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")

		g := gameFrom(ctx)
		if g == nil {
			log.Errorf(ctx, "Controller#Update Game Not Found")
			c.Redirect(http.StatusSeeOther, homePath)
			return
		}
		template, actionType, err := g.Update(ctx)
		switch {
		case err != nil && sn.IsVError(err):
			restful.AddErrorf(ctx, "%v", err)
			withJSON(c, g)
		case err != nil:
			log.Errorf(ctx, err.Error())
			c.Redirect(http.StatusSeeOther, homePath)
			return
		case actionType == game.Cache:
			if err := g.cache(ctx); err != nil {
				restful.AddErrorf(ctx, "%v", err)
			}
		case actionType == game.Save:
			if err := g.save(ctx); err != nil {
				log.Errorf(ctx, "%s", err)
				restful.AddErrorf(ctx, "Controller#Update Save Error: %s", err)
				c.Redirect(http.StatusSeeOther, showPath(prefix, c.Param(hParam)))
				return
			}
		case actionType == game.Undo:
			mkey := g.UndoKey(ctx)
			if err := memcache.Delete(ctx, mkey); err != nil && err != memcache.ErrCacheMiss {
				log.Errorf(ctx, "memcache.Delete error: %s", err)
				c.Redirect(http.StatusSeeOther, showPath(prefix, c.Param(hParam)))
				return
			}
		}

		switch jData := jsonFrom(ctx); {
		case jData != nil && template == "json":
			log.Debugf(ctx, "jData: %v", jData)
			log.Debugf(ctx, "template: %v", template)
			c.JSON(http.StatusOK, jData)
		case template == "":
			log.Debugf(ctx, "template: %v", template)
			c.Redirect(http.StatusSeeOther, showPath(prefix, c.Param(hParam)))
		default:
			log.Debugf(ctx, "template: %v", template)
			cu := user.CurrentFrom(ctx)

			d := gin.H{
				"Context":   ctx,
				"VersionID": info.VersionID(ctx),
				"CUser":     cu,
				"Game":      g,
				"Admin":     game.AdminFrom(ctx),
				"IsAdmin":   user.IsAdmin(ctx),
				"Notices":   restful.NoticesFrom(ctx),
				"Errors":    restful.ErrorsFrom(ctx),
			}
			log.Debugf(ctx, "d: %#v", d)
			c.HTML(http.StatusOK, template, d)
		}
	}
}
func (g *Game) save(ctx context.Context, es ...interface{}) (err error) {
	err = datastore.RunInTransaction(ctx, func(tc context.Context) (terr error) {
		oldG := New(tc)
		if ok := datastore.PopulateKey(oldG.Header, datastore.KeyForObj(tc, g.Header)); !ok {
			terr = fmt.Errorf("Unable to populate game with key.")
			return
		}

		if terr = datastore.Get(tc, oldG.Header); terr != nil {
			return
		}

		if oldG.UpdatedAt != g.UpdatedAt {
			terr = fmt.Errorf("Game state changed unexpectantly.  Try again.")
			return
		}

		if terr = g.encode(ctx); terr != nil {
			return
		}

		if terr = datastore.Put(tc, append(es, g.Header)); terr != nil {
			return
		}

		if terr = memcache.Delete(tc, g.UndoKey(tc)); terr == memcache.ErrCacheMiss {
			terr = nil
		}
		return
	}, &datastore.TransactionOptions{XG: true})
	return
}

func (g *Game) encode(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.TempData = nil
	var encoded []byte
	if encoded, err = codec.Encode(g.State); err != nil {
		return
	}
	g.SavedState = encoded
	g.updateHeader()

	return
}

func (g *Game) cache(ctx context.Context) error {
	item := memcache.NewItem(ctx, g.UndoKey(ctx)).SetExpiration(time.Minute * 30)
	v, err := codec.Encode(g)
	if err != nil {
		return err
	}
	item.SetValue(v)
	return memcache.Set(ctx, item)
}

func wrap(s *stats.Stats, cs contest.Contests) (es []interface{}) {
	es = make([]interface{}, len(cs)+1)
	es[0] = s
	for i, c := range cs {
		es[i+1] = c
	}
	return
}

func showPath(prefix, hid string) string {
	return fmt.Sprintf("/%s/game/show/%s", prefix, hid)
}

func recruitingPath(prefix string) string {
	return fmt.Sprintf("/%s/games/recruiting", prefix)
}

func newPath(prefix string) string {
	return fmt.Sprintf("/%s/game/new", prefix)
}

func newGamer(ctx context.Context) game.Gamer {
	return New(ctx)
}

func Undo(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")
		c.Redirect(http.StatusSeeOther, showPath(prefix, c.Param(hParam)))

		g := gameFrom(ctx)
		if g == nil {
			log.Errorf(ctx, "Controller#Update Game Not Found")
			return
		}
		mkey := g.UndoKey(ctx)
		if err := memcache.Delete(ctx, mkey); err != nil && err != memcache.ErrCacheMiss {
			log.Errorf(ctx, "Controller#Undo Error: %s", err)
		}
	}
}
func Index(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")

		gs := game.GamersFrom(ctx)
		switch status := game.StatusFrom(ctx); status {
		case game.Recruiting:
			c.HTML(http.StatusOK, "shared/invitation_index", gin.H{
				"Context":   ctx,
				"VersionID": info.VersionID(ctx),
				"CUser":     user.CurrentFrom(ctx),
				"Games":     gs,
				"Type":      gType.Indonesia.String(),
			})
		default:
			c.HTML(http.StatusOK, "shared/games_index", gin.H{
				"Context":   ctx,
				"VersionID": info.VersionID(ctx),
				"CUser":     user.CurrentFrom(ctx),
				"Games":     gs,
				"Type":      gType.Indonesia.String(),
				"Status":    status,
			})
		}
	}
}

func NewAction(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")

		g := New(ctx)
		withGame(c, g)
		if err := g.FromParams(ctx, gType.GOT); err != nil {
			log.Errorf(ctx, err.Error())
			c.Redirect(http.StatusSeeOther, recruitingPath(prefix))
			return
		}

		c.HTML(http.StatusOK, prefix+"/new", gin.H{
			"Context":   ctx,
			"VersionID": info.VersionID(ctx),
			"CUser":     user.CurrentFrom(ctx),
			"Game":      g,
		})
	}
}

func Create(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)

		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")
		defer c.Redirect(http.StatusSeeOther, recruitingPath(prefix))

		g := New(ctx)
		withGame(c, g)

		var err error
		if err = g.FromParams(ctx, g.Type); err == nil {
			err = g.encode(ctx)
		}

		if err == nil {
			err = datastore.RunInTransaction(ctx, func(tc context.Context) (err error) {
				if err = datastore.Put(tc, g.Header); err != nil {
					return
				}

				m := mlog.New()
				m.ID = g.ID
				return datastore.Put(tc, m)

			}, &datastore.TransactionOptions{XG: true})
		}

		if err == nil {
			restful.AddNoticef(ctx, "<div>%s created.</div>", g.Title)
		} else {
			log.Errorf(ctx, err.Error())
		}
	}
}

func Accept(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")
		defer c.Redirect(http.StatusSeeOther, recruitingPath(prefix))

		g := gameFrom(ctx)
		if g == nil {
			log.Errorf(ctx, "game not found")
			return
		}

		var (
			start bool
			err   error
		)

		u := user.CurrentFrom(ctx)
		if start, err = g.Accept(ctx, u); err == nil && start {
			err = g.Start(ctx)
		}

		if err == nil {
			err = g.save(ctx)
		}

		if err == nil && start {
			g.SendTurnNotificationsTo(ctx, g.CurrentPlayer())
		}

		if err != nil {
			log.Errorf(ctx, err.Error())
		}

	}
}

func Drop(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")
		defer c.Redirect(http.StatusSeeOther, recruitingPath(prefix))

		g := gameFrom(ctx)
		if g == nil {
			log.Errorf(ctx, "game not found")
			return
		}

		var err error

		u := user.CurrentFrom(ctx)
		if err = g.Drop(u); err == nil {
			err = g.save(ctx)
		}

		if err != nil {
			log.Errorf(ctx, err.Error())
			restful.AddErrorf(ctx, err.Error())
		}

	}
}

func Fetch(c *gin.Context) {
	ctx := restful.ContextFrom(c)
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")
	// create Gamer
	log.Debugf(ctx, "hid: %v", c.Param("hid"))
	id, err := strconv.ParseInt(c.Param("hid"), 10, 64)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	log.Debugf(ctx, "id: %v", id)
	g := New(ctx)
	g.ID = id
	t := g.Type

	switch action := c.PostForm("action"); {
	case action == "reset":
		// pull from memcache/datastore
		// same as undo & !MultiUndo
		fallthrough
	case action == "undo" && !t.MultiUndo():
		// pull from memcache/datastore
		if err := dsGet(ctx, g); err != nil {
			c.Redirect(http.StatusSeeOther, homePath)
			return
		}
	default:
		if user.CurrentFrom(ctx) != nil {
			// pull from memcache and return if successful; otherwise pull from datastore
			if err := mcGet(ctx, g); err == nil {
				return
			}
		}
		log.Debugf(ctx, "g: %#v", g)
		log.Debugf(ctx, "k: %v", datastore.KeyForObj(ctx, g.Header))
		if err := dsGet(ctx, g); err != nil {
			log.Debugf(ctx, "dsGet error: %v", err)
			c.Redirect(http.StatusSeeOther, homePath)
			return
		}
	}
}

// pull temporary game state from memcache.  Note may be different from value stored in datastore.
func mcGet(ctx context.Context, g *Game) error {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	mkey := g.GetHeader().UndoKey(ctx)
	item, err := memcache.GetKey(ctx, mkey)
	if err != nil {
		return err
	}

	if err := codec.Decode(g, item.Value()); err != nil {
		return err
	}

	if err := g.AfterCache(); err != nil {
		return err
	}

	color.WithMap(withGame(restful.GinFrom(ctx), g), g.ColorMapFor(user.CurrentFrom(ctx)))
	return nil
}

// pull game state from memcache/datastore.  returned memcache should be same as datastore.
func dsGet(ctx context.Context, g *Game) error {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch err := datastore.Get(ctx, g.Header); {
	case err != nil:
		restful.AddErrorf(ctx, err.Error())
		return err
	case g == nil:
		err := fmt.Errorf("Unable to get game for id: %v", g.ID)
		restful.AddErrorf(ctx, err.Error())
		return err
	}

	s := newState()
	if err := codec.Decode(&s, g.SavedState); err != nil {
		restful.AddErrorf(ctx, err.Error())
		return err
	} else {
		g.State = s
	}

	if err := g.init(ctx); err != nil {
		log.Debugf(ctx, "g.init error: %v", err)
		restful.AddErrorf(ctx, err.Error())
		return err
	}

	cm := g.ColorMapFor(user.CurrentFrom(ctx))
	log.Debugf(ctx, "cm: %#v", cm)
	color.WithMap(withGame(restful.GinFrom(ctx), g), cm)
	return nil
}

func JSON(c *gin.Context) {
	c.JSON(http.StatusOK, gameFrom(c))
}

func JSONIndexAction(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")

		game.JSONIndexAction(c)
	}
}

func (g *Game) updateHeader() {
	switch g.Phase {
	case GameOver:
		g.Progress = g.PhaseName()
	default:
		g.Progress = fmt.Sprintf("<div>Era: %s | Turn: %d</div><div>Phase: %s</div>", g.Era, g.Turn, g.PhaseName())
	}
	if u := g.Creator; u != nil {
		g.CreatorSID = user.GenID(u.GoogleID)
		g.CreatorName = u.Name
	}

	if l := len(g.Users); l > 0 {
		g.UserSIDS = make([]string, l)
		g.UserNames = make([]string, l)
		g.UserEmails = make([]string, l)
		for i, u := range g.Users {
			g.UserSIDS[i] = user.GenID(u.GoogleID)
			g.UserNames[i] = u.Name
			g.UserEmails[i] = u.Email
		}
	}
}
