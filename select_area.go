package indonesia

import (
	"strconv"
	"strings"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func (g *Game) selectArea(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validateSelectArea(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	cp := g.CurrentPlayer()
	act = game.Cache
	switch g.AdminAction {
	case "admin-header":
		tmpl = "indonesia/admin/header_dialog"
	case "admin-player":
		tmpl = "indonesia/admin/player_dialog"
	case "admin-area":
		tmpl = "indonesia/admin/area_dialog"
	case "admin-company":
		tmpl = "indonesia/admin/company_dialog"
	default:
		switch {
		case cp.CanSelectCard():
			tmpl, err = g.playCard(ctx)
		case cp.CanPlaceCity():
			tmpl, err = g.placeCity(ctx)
		case cp.CanAcquireCompany():
			tmpl, err = g.acquireCompany(ctx)
		case cp.CanResearch():
			tmpl, err = g.conductResearch(ctx)
		case cp.CanSelectCompanyToOperate():
			tmpl, err = g.selectCompany(ctx)
		case cp.CanSelectGood():
			tmpl, err = g.selectGood(ctx)
		case cp.CanSelectShip():
			tmpl, err = g.selectShip(ctx)
		case cp.CanSelectCityOrShip():
			tmpl, err = g.selectCityOrShip(ctx)
		case cp.CanExpandProduction():
			tmpl, err = g.expandProduction(ctx)
		case cp.canExpandShipping():
			tmpl, err = g.expandShipping(ctx)
		case cp.CanAnnounceMerger():
			tmpl, err = g.selectCompany1(ctx)
		case cp.CanAnnounceSecondCompany():
			tmpl, err = g.selectCompany2(ctx)
		case cp.canPlaceInitialProduct():
			tmpl, err = g.placeInitialProduct(ctx)
		case cp.canPlaceInitialShip():
			tmpl, err = g.placeInitialShip(ctx)
		case cp.CanCreateSiapFaji():
			tmpl, err = g.removeRiceSpice(ctx)
		default:
			tmpl = "indonesia/flash_notice"
			act = game.None
			err = sn.NewVError("Can't find action for selection.")
		}
	}
	return
}

func (g *Game) validateSelectArea(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if !g.CUserIsCPlayerOrAdmin(ctx) {
		err = sn.NewVError("Only the current player can perform an action.")
		return
	}

	var i, id, slot int
	areaID := restful.GinFrom(ctx).PostForm("area")

	switch splits := strings.Split(areaID, "-"); {
	case splits[0] == "admin" && splits[1] == "area":
		g.AdminAction = "admin-area"
		if id, err = strconv.Atoi(splits[2]); err == nil {
			g.SelectedAreaID = AreaID(id)
		}
	case splits[0] == "admin" && splits[1] == "player":
		g.AdminAction = "admin-player"
		if id, err = strconv.Atoi(splits[2]); err == nil {
			g.SelectedPlayerID = id
		}
	case splits[0] == "admin" && splits[1] == "company":
		g.AdminAction = "admin-company"
		if id, err = strconv.Atoi(splits[2]); err == nil {
			g.SelectedPlayerID = id

			if slot, err = strconv.Atoi(splits[3]); err == nil {
				g.SelectedSlot = slot
			}
		}
	case splits[0] == "admin":
		g.AdminAction = areaID
	case splits[0] == "card":
		if i, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedCardIndex = i
		}
	case splits[0] == "available":
		if i, err = strconv.Atoi(splits[2]); err == nil {
			g.SelectedDeedIndex = i
		}
	case splits[0] == "area":
		if id, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedAreaID = AreaID(id)
		}
	case splits[0] == "research":
		if i, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedTechnology = Technology(i)
		}
	case splits[0] == "company":
		if i, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedSlot = i
			g.setSelectedPlayer(g.CurrentPlayer())
		}
	case splits[0] == "ship":
		if i, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedArea2ID = AreaID(i)

			if i, err = strconv.Atoi(splits[2]); err == nil {
				g.SelectedShipperIndex = i
			}
		}
	case splits[0] == "city":
		if i, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedArea2ID = AreaID(i)
		}
	case splits[0] == "player":
		if id, err = strconv.Atoi(splits[1]); err == nil {
			g.SelectedPlayerID = id

			if slot, err = strconv.Atoi(splits[3]); err == nil {
				g.SelectedSlot = slot
			}
		}
	default:
		err = sn.NewVError("Unable to determine selection.")
	}
	return
}
