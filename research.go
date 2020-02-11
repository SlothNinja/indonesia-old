package indonesia

import (
	"encoding/gob"
	"html/template"
	"strings"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(researchEntry))
}

type Technology int
type Technologies map[Technology]int

const (
	NoTech Technology = iota
	BidMultiplierTech
	SlotsTech
	MergersTech
	ExpansionsTech
	HullTech
)

var technologyStrings = map[Technology]string{
	NoTech:            "None",
	BidMultiplierTech: "Turn Order Bid",
	SlotsTech:         "Slots",
	MergersTech:       "Mergers",
	ExpansionsTech:    "Expansions",
	HullTech:          "Hull",
}

func (t Technology) String() string {
	return technologyStrings[t]
}

func (t Technology) Int() int {
	return int(t)
}

func (t Technology) IDString() string {
	return strings.Replace(strings.ToLower(t.String()), " ", "-", -1)
}

func (p *Player) Expansions() int {
	return p.Technologies[ExpansionsTech]
}

func (g *Game) startResearch(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = Research
	g.beginningOfPhaseReset()
	g.setCurrentPlayers(g.Players()[0])
}

func (g *Game) conductResearch(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var tech Technology

	cp := g.CurrentPlayer()
	switch tech, err = g.validateConductResearch(ctx); {
	case err != nil:
	case tech == HullTech:
		g.SubPhase = RSelectPlayer
		tmpl = "indonesia/select_hull_player_dialog"
	case tech == SlotsTech:
		cp.Slots[cp.Technologies[SlotsTech]].Developed = true
		fallthrough
	default:
		cp.Technologies[tech] += 1
		cp.PerformedAction = true

		// Log
		e := g.newResearchEntryFor(cp, nil, tech, cp.Technologies[tech])
		restful.AddNoticef(ctx, string(e.HTML(ctx)))
		tmpl = "indonesia/research_update"
	}
	return
}

func (g *Game) validateConductResearch(ctx context.Context) (tech Technology, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var cp *Player

	switch cp, tech, err = g.CurrentPlayer(), g.SelectedTechnology, g.validatePlayerAction(ctx); {
	case err != nil:
	case tech < BidMultiplierTech || tech > HullTech:
		err = sn.NewVError("Received invalid for researched technology.")
	case tech != HullTech && cp.Technologies[tech] == 5:
		err = sn.NewVError("Your %s is already at the maximum level.", tech)
	}
	return
}

type researchEntry struct {
	*Entry
	Technology Technology
	Level      int
}

func (g *Game) newResearchEntryFor(p, op *Player, t Technology, l int) (e *researchEntry) {
	if t == BidMultiplierTech {
		l = bidMultiplier[p.Technologies[BidMultiplierTech]]
	}

	e = &researchEntry{
		Entry:      g.newEntryFor(p),
		Technology: t,
		Level:      l,
	}
	if op != nil {
		e.SetOtherPlayer(op)
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *researchEntry) HTML(ctx context.Context) (s template.HTML) {
	g := gameFrom(ctx)
	n := g.NameByPID(e.PlayerID)
	if e.OtherPlayerID == NoPlayerID {
		return restful.HTML("<div>%s increased %s to %d</div>", n, e.Technology, e.Level)
	} else {
		return restful.HTML("<div>%s increased %s of %s to %d</div>", n, e.Technology,
			g.NameByPID(e.OtherPlayerID), e.Level)
	}
}

func (g *Game) selectHullPlayer(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var p *Player

	if p, err = g.validateSelectHullPlayer(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	cp := g.CurrentPlayer()
	p.Technologies[HullTech] += 1
	cp.PerformedAction = true

	// Log
	if cp.Equal(p) {
		e := g.newResearchEntryFor(cp, nil, HullTech, p.Technologies[HullTech])
		restful.AddNoticef(ctx, string(e.HTML(ctx)))
	} else {
		e := g.newResearchEntryFor(cp, p, HullTech, p.Technologies[HullTech])
		restful.AddNoticef(ctx, string(e.HTML(ctx)))
	}
	tmpl, act = "indonesia/research_update", game.Cache
	return
}

func (g *Game) validateSelectHullPlayer(ctx context.Context) (p *Player, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if !g.CUserIsCPlayerOrAdmin(ctx) {
		err = sn.NewVError("Only the current player can perform an action.")
		return
	}

	switch p = g.PlayerBySID(restful.GinFrom(ctx).PostForm("id")); {
	case p == nil:
		err = sn.NewVError("Received invalid player.")
	case p.Technologies[HullTech] == 5:
		err = sn.NewVError("Hull size of %s is already 5.", g.NameFor(p))
	}
	return
}
