package indonesia

import (
	"encoding/gob"
	"html/template"
	"sort"
	"strconv"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(bidEntry))
	gob.Register(new(turnOrderEntry))
}

const NoBid = -1

func (g *Game) startBidForTurnOrder(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = BidForTurnOrder
	g.setCurrentPlayers(g.Players()[0])
}

func (g *Game) placeTurnOrderBid(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validateBid(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	cp := g.CurrentPlayer()
	cp.Bank += cp.Bid
	cp.Rupiah -= cp.Bid
	cp.PerformedAction = true

	// Log placement
	e := g.newBidEntryFor(cp)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	tmpl, act = "indonesia/turn_order_bid_update", game.Cache
	return
}

func (g *Game) validateBid(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch err = g.validatePlayerAction(ctx); {
	case err != nil:
	default:
		cp := g.CurrentPlayer()
		switch cp.Bid, err = strconv.Atoi(restful.GinFrom(ctx).PostForm("Bid")); {
		case err != nil:
		case cp.Bid > cp.Rupiah:
			err = sn.NewVError("You bid more than you have.")
		case cp.Bid < 0:
			err = sn.NewVError("You can't bid less than zero.")
		}
	}
	return
}

type bidEntry struct {
	*Entry
	Bid           int
	BidMultiplier int
}

func (g *Game) newBidEntryFor(p *Player) (e *bidEntry) {
	e = &bidEntry{
		Entry:         g.newEntryFor(p),
		Bid:           p.Bid,
		BidMultiplier: p.Multiplier(),
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *bidEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("<div>%s bid %d &times; %d for a total bid of %d</div>",
		g.NameByPID(e.PlayerID), e.Bid, e.BidMultiplier, e.Bid*e.BidMultiplier)
}

func (g *Game) setTurnOrder(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c, n := make([]int, g.NumPlayers), make([]int, g.NumPlayers)
	for i, p := range g.Players() {
		c[i] = p.ID()
	}

	ps := g.Players()
	b := make([]int, g.NumPlayers)
	sort.Sort(Reverse{ByTurnOrderBid{ps}})
	g.setPlayers(ps)
	cp := g.Players()[0]
	g.setCurrentPlayers(cp)

	// Log new order
	for i, p := range g.Players() {
		pid := p.ID()
		n[i] = pid
		b[pid] = p.TotalBid()
	}
	g.newTurnOrderEntry(c, n, b)
	g.startMergers(ctx)
}

type turnOrderEntry struct {
	*Entry
	Current []int
	New     []int
	Bids    []int
}

func (g *Game) newTurnOrderEntry(c, n, b []int) {
	e := &turnOrderEntry{
		Entry:   g.newEntry(),
		Current: c,
		New:     n,
		Bids:    b,
	}
	g.Log = append(g.Log, e)
}

func (e *turnOrderEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	s := restful.HTML("<div><table class='strippedDataTable'><thead><tr><th>Player</th><th>Bid</th></tr></thead><tbody>")
	for _, pid := range e.Current {
		s += restful.HTML("<tr><td>%s</td><td>%d</td></tr>", g.NameByPID(pid), e.Bids[pid])
	}
	s += restful.HTML("</tbody></table></div>")
	names := make([]string, g.NumPlayers)
	for i, pid := range e.New {
		names[i] = g.NameByPID(pid)
	}
	s += restful.HTML("<div class='top-padding'>New Turn Order: %s.</div>", restful.ToSentence(names))
	return s
}
