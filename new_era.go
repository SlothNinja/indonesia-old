package indonesia

import (
	"encoding/gob"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(noNewEraEntry))
	gob.Register(new(newEraEntry))
	gob.Register(new(endGameTriggeredEntry))
}

func (g *Game) startNewEra(ctx context.Context) (cs contest.Contests) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = NewEra
	g.Turn += 1
	g.Round = 1
	g.beginningOfPhaseReset()
	g.resetCompanies()
	g.resetCities()
	cs = g.checkForNewEra(ctx)
	return
}

//func (g *Game) beginningOfTurnReset() {
//	g.beginningOfPhaseReset()
//	for _, p := range g.Players() {
//		p.OpIncome = 0
//	}
//}

func (g *Game) checkForNewEra(ctx context.Context) (cs contest.Contests) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.AvailableDeeds = g.AvailableDeeds.RemoveUnstartable(g)
	switch n := g.AvailableDeeds.Types(); {
	case n < 2 && g.Era != EraC:
		g.Era += 1
		g.newNewEraEntry(n, g.Era, g.AvailableDeeds)
		g.AvailableDeeds = deedsFor(g.Era).RemoveUnstartable(g)
		g.startNewCity(ctx)
	case n < 2 && g.Era == EraC:
		g.newEndGameTriggeredEntry(n)
		cs = g.endGame(ctx)
	default:
		g.newNoNewEraEntry(n, g.Era)
		g.startBidForTurnOrder(ctx)
	}
	return
}

func (g *Game) startNewCity(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")
}

type newEraEntry struct {
	*Entry
	Types int
	Era   Era
	Deeds Deeds
}

func (g *Game) newNewEraEntry(types int, era Era, deeds Deeds) *newEraEntry {
	e := &newEraEntry{
		Entry: g.newEntry(),
		Types: types,
		Era:   era,
		Deeds: deeds,
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *newEraEntry) HTML(ctx context.Context) (s template.HTML) {
	switch e.Types {
	case 0:
		s = restful.HTML("<div>No deeds available for acquistion.</div>")
	default:
		s += restful.HTML("<div>Only one type of deed available for acquistion.</div>")
		for _, deed := range e.Deeds {
			s += restful.HTML("<div>%s %s deed discarded.</div>", deed.Province, deed.Goods)
		}
	}
	s += restful.HTML("<div>Era %q begins.</div>", e.Era)
	return
}

type noNewEraEntry struct {
	*Entry
	Types int
	Era   Era
}

func (g *Game) newNoNewEraEntry(types int, era Era) *noNewEraEntry {
	e := &noNewEraEntry{
		Entry: g.newEntry(),
		Types: types,
		Era:   era,
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *noNewEraEntry) HTML(ctx context.Context) (s template.HTML) {
	s = restful.HTML("<div>%d types of deeds available for acquistion.</div>", e.Types)
	s += restful.HTML("<div>Era %q continues.</div>", e.Era)
	return
}

type endGameTriggeredEntry struct {
	*Entry
	Types int
}

func (g *Game) newEndGameTriggeredEntry(types int) *endGameTriggeredEntry {
	e := &endGameTriggeredEntry{
		Entry: g.newEntry(),
		Types: types,
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *endGameTriggeredEntry) HTML(ctx context.Context) (s template.HTML) {
	switch e.Types {
	case 0:
		s = restful.HTML("<div>No deeds available for acquistion in Era \"c\".</div>")
	default:
		s = restful.HTML("<div>Only one type of deed available for acquistion in Era \"c\".</div>")
	}
	s += restful.HTML("<div>End of game triggered.</div>")
	return
}
