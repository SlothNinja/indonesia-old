package indonesia

import (
	"net/http"
	"time"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user/stats"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/context"
)

func Finish(prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := restful.ContextFrom(c)
		log.Debugf(ctx, "Entering")
		defer log.Debugf(ctx, "Exiting")
		defer c.Redirect(http.StatusSeeOther, showPath(prefix, c.Param(hParam)))

		g := gameFrom(ctx)
		oldCP := g.CurrentPlayer()

		var (
			s   *stats.Stats
			cs  contest.Contests
			err error
		)

		if s, cs, err = g.finishTurn(ctx); err != nil {
			log.Errorf(ctx, err.Error())
			return
		}

		// Game is over if cs != nil
		if cs != nil {
			g.Phase = GameOver
			g.Status = game.Completed
			if err = g.save(ctx, wrap(s.GetUpdate(ctx, time.Time(g.UpdatedAt)), cs)...); err == nil {
				err = g.SendEndGameNotifications(ctx)
			}
		} else {
			if err = g.save(ctx, s.GetUpdate(ctx, time.Time(g.UpdatedAt))); err == nil {
				if newCP := g.CurrentPlayer(); newCP != nil && oldCP.ID() != newCP.ID() {
					err = g.SendTurnNotificationsTo(ctx, newCP)
				}
			}
		}

		if err != nil {
			log.Errorf(ctx, err.Error())
		}

		return
	}
}

func (g *Game) finishTurn(ctx context.Context) (s *stats.Stats, cs contest.Contests, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch {
	case g.Phase == NewEra:
		s, err = g.newEraFinishTurn(ctx)
	case g.Phase == BidForTurnOrder:
		s, err = g.bidForTurnOrderFinishTurn(ctx)
	case g.Phase == Mergers && g.SubPhase == MBid:
		s, err = g.mergersBidFinishTurn(ctx)
	case g.Phase == Mergers:
		s, err = g.mergersFinishTurn(ctx)
	case g.Phase == Acquisitions:
		s, err = g.acquisitionsFinishTurn(ctx)
	case g.Phase == Research:
		s, cs, err = g.researchFinishTurn(ctx)
	case g.Phase == Operations:
		s, cs, err = g.companyExpansionFinishTurn(ctx)
	case g.Phase == CityGrowth:
		s, cs, err = g.cityGrowthFinishTurn(ctx)
	default:
		err = sn.NewVError("Improper Phase for finishing turn.")
	}

	return
}

func (g *Game) validateFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var cp *Player

	switch cp, s = g.CurrentPlayer(), stats.Fetched(ctx); {
	case s == nil:
		err = sn.NewVError("missing stats for player.")
	case !g.CUserIsCPlayerOrAdmin(ctx):
		err = sn.NewVError("Only the current player may finish a turn.")
	case !cp.PerformedAction:
		err = sn.NewVError("%s has yet to perform an action.", g.NameFor(cp))
	}
	return
}

// ps is an optional parameter.
// If no player is provided, assume current player.
func (g *Game) nextPlayer(ps ...game.Playerer) (p *Player) {
	ctx := g.CTX()
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if nper := g.NextPlayerer(ps...); nper != nil {
		p = nper.(*Player)
	}
	return
}

func (g *Game) newEraNextPlayer(pers ...game.Playerer) (p *Player) {
	ctx := g.CTX()
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.CurrentPlayer().endOfTurnUpdate()
	p = g.nextPlayer(pers...)
	for g.Players().anyCanPlaceCity() {
		if !p.CanPlaceCity() {
			p = g.nextPlayer(p)
		} else {
			p.beginningOfTurnReset()
			return
		}
	}
	return nil
}

func (g *Game) removeUnplayableCityCardsFor(ctx context.Context, p *Player) {
	var newCityCards CityCards
	for _, card := range p.CityCards {
		if card.Era != g.Era {
			newCityCards = append(newCityCards, card)
		} else {
			e := g.newDiscardCityEntryFor(p, card)
			restful.AddNoticef(ctx, string(e.HTML(ctx)))
		}
	}
	p.CityCards = newCityCards
}

func (g *Game) newEraFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateNewEraFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.newEraNextPlayer(); np == nil {
		for _, p := range g.Players() {
			g.removeUnplayableCityCardsFor(ctx, p)
		}
		g.startBidForTurnOrder(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateNewEraFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateFinishTurn(ctx); g.Phase != NewEra {
		err = sn.NewVError(`Expected "New Era" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) bidForTurnOrderNextPlayer(pers ...game.Playerer) *Player {
	g.CurrentPlayer().endOfTurnUpdate()
	p := g.nextPlayer(pers...)
	for !p.Equal(g.Players()[0]) {
		if !p.CanBid() {
			p = g.nextPlayer(p)
		} else {
			return p
		}
	}
	return nil
}

func (g *Game) bidForTurnOrderFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateBidForTurnOrderFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.bidForTurnOrderNextPlayer(); np == nil {
		g.setTurnOrder(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateBidForTurnOrderFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	if s, err = g.validateFinishTurn(ctx); g.Phase != BidForTurnOrder {
		err = sn.NewVError(`Expected "Bid For Turn Order" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) mergersBidNextPlayer(pers ...game.Playerer) *Player {
	g.CurrentPlayer().endOfTurnUpdate()
	p := g.nextPlayer(pers...)
	for !g.Players().allPassed() {
		if !p.CanBidOnMerger() {
			g.autoPass(p)
			p = g.nextPlayer(p)
		} else {
			return p
		}
	}
	return nil
}

func (g *Game) mergersBidFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateMergersBidFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.mergersBidNextPlayer(); np == nil {
		g.startMergerResolution(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateMergersBidFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	if s, err = g.validateFinishTurn(ctx); g.Phase != Mergers {
		err = sn.NewVError(`Expected "Mergers" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) mergersNextPlayer(pers ...game.Playerer) *Player {
	g.CurrentPlayer().endOfTurnUpdate()
	p := g.nextPlayer(pers...)
	for !g.Players().allPassed() {
		if !p.CanAnnounceMerger() {
			g.autoPass(p)
			p = g.nextPlayer(p)
		} else {
			return p
		}
	}
	return nil
}

func (g *Game) mergersFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateMergersFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if g.SubPhase == MSiapFajiCreation {
		announcer := g.PlayerByID(g.Merger.AnnouncerID)
		g.Merger = nil
		g.setCurrentPlayers(announcer)
		g.beginningOfPhaseReset()
		g.SubPhase = MSelectCompany1
		if np := g.mergersNextPlayer(); np != nil {
			g.setCurrentPlayers(np)
		} else {
			g.startAcquisitions(ctx)
		}
	} else {
		if np := g.mergersNextPlayer(); np == nil {
			g.startAcquisitions(ctx)
		} else {
			g.setCurrentPlayers(np)
		}
	}
	return
}

func (g *Game) validateMergersFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateFinishTurn(ctx); g.Phase != Mergers {
		err = sn.NewVError(`Expected "Mergers" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) acquisitionsNextPlayer(pers ...game.Playerer) (p *Player) {
	g.CurrentPlayer().endOfTurnUpdate()
	p = g.nextPlayer(pers...)
	for !g.Players().allPassed() {
		if !p.CanAcquireCompany() {
			g.autoPass(p)
			p = g.nextPlayer(p)
		} else {
			return
		}
	}
	p = nil
	return
}

func (g *Game) acquisitionsFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateAcquisitionsFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.acquisitionsNextPlayer(); np == nil {
		g.startResearch(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateAcquisitionsFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateFinishTurn(ctx); g.Phase != Acquisitions {
		err = sn.NewVError(`Expected "Acquisitions" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) researchNextPlayer(pers ...game.Playerer) *Player {
	g.CurrentPlayer().endOfTurnUpdate()
	p := g.nextPlayer(pers...)
	for !p.Equal(g.Players()[0]) {
		if !p.CanResearch() {
			g.autoPass(p)
			p = g.nextPlayer(p)
		} else {
			return p
		}
	}
	return nil
}

func (g *Game) researchFinishTurn(ctx context.Context) (s *stats.Stats, cs contest.Contests, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateResearchFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.researchNextPlayer(); np == nil {
		cs = g.startOperations(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateResearchFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateFinishTurn(ctx); g.Phase != Research {
		err = sn.NewVError(`Expected "Research" phase but have %q phase.`, g.Phase)
	}
	return
}

func (g *Game) companyExpansionNextPlayer(pers ...game.Playerer) *Player {
	g.CurrentPlayer().endOfTurnUpdate()
	p := g.nextPlayer(pers...)
	g.OverrideDeliveries = -1
	for !g.AllCompaniesOperated() {
		if !p.HasCompanyToOperate() {
			g.autoPass(p)
			p = g.nextPlayer(p)
		} else {
			return p
		}
	}
	return nil
}

func (g *Game) companyExpansionFinishTurn(ctx context.Context) (s *stats.Stats, cs contest.Contests, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateCompanyExpansionFinishTurn(ctx); err != nil {
		return
	}

	restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))

	if np := g.companyExpansionNextPlayer(); np == nil {
		cs = g.startCityGrowth(ctx)
	} else {
		g.Phase = Operations
		g.SubPhase = OPSelectCompany
		g.resetShipping()
		np.beginningOfTurnReset()
		g.setCurrentPlayers(np)
	}
	return
}

func (g *Game) validateCompanyExpansionFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c := g.SelectedCompany()
	switch s, err = g.validateFinishTurn(ctx); {
	case err != nil:
	case g.Phase != Operations:
		err = sn.NewVError("Expected %q phase but have %q phase.", Operations, g.PhaseName())
	case g.SubPhase != OPFreeExpansion && g.SubPhase != OPExpansion:
		err = sn.NewVError("Expected an expansion subphase but have %q subphase.", g.SubPhaseName())
	case c == nil:
		err = sn.NewVError("You must select a company to operate.")
	case !c.Operated:
		err = sn.NewVError("You must operate the selected company.")
	}
	return
}

func (g *Game) cityGrowthFinishTurn(ctx context.Context) (s *stats.Stats, cs contest.Contests, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if s, err = g.validateCityGrowthFinishTurn(ctx); err == nil {
		restful.AddNoticef(ctx, "%s finished turn.", g.NameFor(g.CurrentPlayer()))
		cs = g.startNewEra(ctx)
	}
	return
}

func (g *Game) validateCityGrowthFinishTurn(ctx context.Context) (s *stats.Stats, err error) {
	cmap := g.CityGrowthMap()
	switch s, err = g.validateFinishTurn(ctx); {
	case err != nil:
	case g.Phase != CityGrowth:
		err = sn.NewVError("Expected %q phase but have %q phase.", CityGrowth, g.PhaseName())
	case g.C3StonesToUse(cmap) > 0:
		err = sn.NewVError("You did not select enough size 2 cities to grow.")
	case g.C2StonesToUse(cmap) > 0:
		err = sn.NewVError("You did not select enough size 1 cities to grow.")
	}
	return
}
