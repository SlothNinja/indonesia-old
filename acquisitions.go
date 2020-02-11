package indonesia

import (
	"encoding/gob"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(acquiredCompanyEntry))
}

func (g *Game) startAcquisitions(ctx context.Context) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = Acquisitions
	g.beginningOfPhaseReset()
	if np := g.acquisitionsNextPlayer(g.Players()[g.NumPlayers-1]); np == nil {
		g.startResearch(ctx)
	} else {
		g.setCurrentPlayers(np)
	}
}

func (g *Game) SelectedCompany() *Company {
	if p, slot := g.SelectedPlayer(), g.SelectedSlot; p == nil || slot == NoSlot || slot < 1 || slot > 5 {
		return nil
	} else {
		return p.Slots[slot-1].Company
	}
}

func (g *Game) SelectedShippingCompany() *Company {
	return g.ShippingCompanies()[g.SelectedShippingProvince]
}

func (g *Game) acquireCompany(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		s              *Slot
		d              *Deed
		sIndex, dIndex int
	)

	if s, sIndex, d, dIndex, err = g.validateAcquireCompany(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	cp := g.CurrentPlayer()
	s.Company = newCompany(g, cp, sIndex, d)

	// Cache SelectedSlot, SelectedPlayerID so SelectedCompany works.
	g.setSelectedPlayer(cp)
	g.SelectedSlot = sIndex
	g.AvailableDeeds = g.AvailableDeeds.removeAt(dIndex)
	if s.Company.IsProductionCompany() {
		g.SubPhase = AQInitialProduction
	} else {
		s.Company.ShipType = g.getAvailableShipType()
		g.SubPhase = AQInitialShip
	}
	tmpl = "indonesia/acquire_company_update"
	return
}

func (g *Game) validateAcquireCompany(ctx context.Context) (s *Slot, sIndex int, d *Deed, dIndex int, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validatePlayerAction(ctx); err != nil {
		return
	}

	cp := g.CurrentPlayer()
	s, sIndex = cp.getEmptySlot()
	d, dIndex = g.SelectedDeed(), g.SelectedDeedIndex

	switch {
	case d == nil:
		err = sn.NewVError("You must select deed.")
	case s == nil:
		err = sn.NewVError("You do not have a free slot for the company.")
	}
	return
}

func (g *Game) placeInitialProduct(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		a *Area
		c *Company
	)

	if a, c, err = g.validateplaceInitialProduct(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	cp := g.CurrentPlayer()
	cp.PerformedAction = true
	a.AddProducer(c)
	c.AddArea(a)

	// Log placement
	e := g.newAcquiredCompanyEntryFor(cp, c)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))

	// Reset SubPhase
	g.SubPhase = NoSubPhase
	tmpl = "indonesia/placed_product_update"
	return
}

func (g *Game) validateplaceInitialProduct(ctx context.Context) (a *Area, c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch a, c, err = g.SelectedArea(), g.SelectedCompany(), g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("You must acquire a company first.")
	case a == nil:
		err = sn.NewVError("You must select an area for the %s token.", c.Goods())
	case !a.IsLand():
		err = sn.NewVError("You must select a land area for the initial %s token.", c.Goods())
	case c.Deeds[0].Province != a.Province():
		err = sn.NewVError("You must select a land area in the %s province for the initial %s token.", c.Deeds[0].Province, c.Goods())
	case a.City != nil:
		err = sn.NewVError("You can not place a %s token in an area having a city.", c.Goods())
	case a.Producer != nil:
		err = sn.NewVError("You can not place a %s token in an area already having goods token.", c.Goods())
	case a.adjacentAreaHasCompetingCompanyFor(c):
		err = sn.NewVError("You can not place a %s token adjacent to an area having %s token.", c.Goods(), c.Goods())
	}
	return
}

type acquiredCompanyEntry struct {
	*Entry
	Deed Deed
}

func (g *Game) newAcquiredCompanyEntryFor(p *Player, company *Company) (e *acquiredCompanyEntry) {
	e = &acquiredCompanyEntry{
		Entry: g.newEntryFor(p),
		Deed:  *(company.Deeds[0]),
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *acquiredCompanyEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("<div>%s started a %s company in the %s province.</div>",
		g.NameByPID(e.PlayerID), e.Deed.Goods, e.Deed.Province)
}

func (g *Game) placeInitialShip(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		a *Area
		c *Company
	)

	if a, c, err = g.validateplaceInitialShip(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	cp := g.CurrentPlayer()
	cp.PerformedAction = true
	c.AddShipIn(a)

	// Log placement
	e := g.newAcquiredCompanyEntryFor(cp, c)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))

	// Reset SubPhase
	g.SubPhase = NoSubPhase
	tmpl = "indonesia/placed_product_update"
	return
}

func (g *Game) validateplaceInitialShip(ctx context.Context) (a *Area, c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validatePlayerAction(ctx); err != nil {
		return
	}

	switch a, c = g.SelectedArea(), g.SelectedCompany(); {
	case c == nil:
		err = sn.NewVError("You must acquire a company first.")
	case a == nil:
		err = sn.NewVError("You must select an area for the %s token.", c.Goods)
	case !a.IsSea():
		err = sn.NewVError("You must select a sea area.")
	case !a.adjacentToProvince(c.Deeds[0].Province):
		err = sn.NewVError("You must select a sea are adjacent to the %s province.", c.Deeds[0].Province)
	}
	return
}
