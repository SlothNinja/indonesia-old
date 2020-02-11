package indonesia

import (
	"encoding/gob"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(passEntry))
	gob.Register(new(autoPassEntry))
}

func (g *Game) pass(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validatePass(ctx); err != nil {
		log.Debugf(ctx, err.Error())
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	cp := g.CurrentPlayer()
	cp.Passed = true
	cp.PerformedAction = true

	// Log Pass
	e := g.newPassEntryFor(cp)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))

	tmpl, act = "indonesia/pass_update", game.Cache
	return
}

func (g *Game) validatePass(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validatePlayerAction(ctx); err != nil {
		return
	}

	switch {
	case g.Phase == Acquisitions && g.SubPhase != NoSubPhase:
		err = sn.NewVError("You can not pass in SubPhase: %v", g.SubPhaseName())
	case g.Phase == Mergers && g.SubPhase != MSelectCompany1:
		err = sn.NewVError("You cannot pass in SubPhase: %v", g.SubPhaseName())
	case g.Phase != Acquisitions && g.Phase != Mergers:
		err = sn.NewVError("You cannot pass in Phase: %v", g.PhaseName())
	}
	return
}

type passEntry struct {
	*Entry
}

func (g *Game) newPassEntryFor(p *Player) (e *passEntry) {
	e = &passEntry{
		Entry: g.newEntryFor(p),
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *passEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("%s passed.", g.NameByPID(e.PlayerID))
}

func (g *Game) autoPass(p *Player) {
	p.PerformedAction = true
	p.Passed = true
	g.newAutoPassEntryFor(p)
}

type autoPassEntry struct {
	*Entry
}

func (g *Game) newAutoPassEntryFor(p *Player) (e *autoPassEntry) {
	e = new(autoPassEntry)
	e.Entry = g.newEntryFor(p)
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *autoPassEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("System auto passed for %s.", g.NameByPID(e.PlayerID))
}
