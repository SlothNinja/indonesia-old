package indonesia

import (
	"encoding/gob"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(selectCompanyEntry))
	gob.Register(new(deliveredGoodEntry))
	gob.Register(new(receiveIncomeEntry))
	gob.Register(make(ShipperIncomeMap, 0))
	gob.Register(new(expandProductionEntry))
	gob.Register(new(expandShippingEntry))
	gob.Register(new(stopExpandingEntry))
}

type ShipperIncomeMap map[int]int

func (m ShipperIncomeMap) OtherShips(pid int) int {
	ships := 0
	for id, s := range m {
		if id != pid {
			ships += s
		}
	}
	return ships
}

func (m ShipperIncomeMap) OwnShips(pid int) int {
	ships := 0
	for id, s := range m {
		if id == pid {
			ships += s
		}
	}
	return ships
}

func (g *Game) startOperations(ctx context.Context) (cs contest.Contests) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var np *Player

	if np = g.companyExpansionNextPlayer(); np == nil {
		cs = g.startCityGrowth(ctx)
		return
	}

	g.beginningOfPhaseReset()
	g.Phase = Operations
	g.SubPhase = OPSelectCompany
	g.resetShipping()
	g.resetOpIncome()
	g.setCurrentPlayers(np)
	g.OverrideDeliveries = -1
	return
}

func (g *Game) resetOpIncome() {
	for _, p := range g.Players() {
		p.OpIncome = 0
	}
}

func (g *Game) selectCompany(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var c *Company
	switch c, err = g.validateSelectCompany(ctx); {
	case err != nil:
		tmpl = "indonesia/flash_notice"
	case c.IsShippingCompany():
		g.SubPhase = OPFreeExpansion
		if cp := g.CurrentPlayer(); cp.canExpandShipping() {

			// Log
			e := g.newSelectCompanyEntryFor(g.CurrentPlayer(), c, 0)
			restful.AddNoticef(ctx, string(e.HTML(ctx)))
			tmpl = "indonesia/select_company_update"
		} else {
			cp.PerformedAction = true
			c.Operated = true
			e := g.newSelectCompanyEntryFor(g.CurrentPlayer(), c, 0)
			restful.AddNoticef(ctx, string(e.HTML(ctx)))
			tmpl = "indonesia/completed_expansion_dialog"
		}
	default:
		e := g.newSelectCompanyEntryFor(g.CurrentPlayer(), c, 0)
		restful.AddNoticef(ctx, string(e.HTML(ctx)))
		if g.OverrideDeliveries > -1 {
			g.RequiredDeliveries = g.OverrideDeliveries
		} else {
			g.RequiredDeliveries, g.ProposedPath = c.maxFlow()
		}
		if g.RequiredDeliveries > 0 {
			g.SubPhase = OPSelectProductionArea
			g.ShipperIncomeMap = make(ShipperIncomeMap, 0)
			tmpl = "indonesia/select_company_update"
		} else {
			tmpl = g.startCompanyExpansion(ctx)
		}
	}
	return
}

func (g *Game) validateSelectCompany(ctx context.Context) (c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if c, err = g.SelectedCompany(), g.validatePlayerAction(ctx); c == nil {
		err = sn.NewVError("Missing company selection.")
	}
	return
}

type selectCompanyEntry struct {
	*Entry
	Company Company
	Deliver int
}

func (g *Game) newSelectCompanyEntryFor(p *Player, c *Company, d int) (e *selectCompanyEntry) {
	e = &selectCompanyEntry{
		Entry:   g.newEntryFor(p),
		Company: *c,
		Deliver: d,
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *selectCompanyEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	company := e.Company
	name := g.NameByPID(e.PlayerID)
	return restful.HTML("<div>%s selected the %s company to operate.</div>", name, company.String())
}

func (g *Game) selectGood(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var a *Area
	if a, err = g.validateSelectGood(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	g.SubPhase = OPSelectShip
	g.SelectedGoodsAreaID = a.ID
	g.SelectedShippingProvince = NoProvince
	a.Used = true
	from := sourceFID
	to := toFlowID(a.ID)
	g.CustomPath = g.CustomPath.addFlow(from, to)
	tmpl = "indonesia/select_good_update"
	return
}

const (
	shipInput int = iota + 1
	shipOutput
)

func (fp flowMatrix) addFlow(source, target FlowID) flowMatrix {
	var flowPath flowMatrix
	if fp == nil {
		flowPath = make(flowMatrix, 0)
	} else {
		flowPath = fp
	}
	if flowPath[source] == nil {
		flowPath[source] = make(subflow, 0)
	}
	if flowPath[target] == nil {
		flowPath[target] = make(subflow, 0)
	}
	flowPath[source][target] += 1
	flowPath[target][source] -= 1
	return flowPath
}

func (g *Game) validateSelectGood(ctx context.Context) (a *Area, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if err = g.validatePlayerAction(ctx); err != nil {
		return
	}

	c := g.SelectedCompany()
	a = g.SelectedArea()
	zone := c.ZoneFor(a)

	switch {
	case c == nil:
		err = sn.NewVError("You must select company to operate.")
	case a == nil:
		err = sn.NewVError("You must select a good area.")
	case zone == nil:
		err = sn.NewVError("You must select a good in a production zone of the company.")
	case a.Used:
		err = sn.NewVError("The selected area has already delivered its goods.")
	}
	return
}

const InvalidUsedShips = -1

func (g *Game) selectShip(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		old, area *Area
		shipper   *Shipper
		incomeMap ShipperIncomeMap
	)

	if old, area, shipper, incomeMap, err = g.validateSelectShip(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}
	area.Used = true
	shipper.Delivered += 1
	if g.ShipsUsed == InvalidUsedShips {
		g.ShipsUsed = 1
	} else {
		g.ShipsUsed += 1
	}

	province := shipper.Province()
	g.SelectedShippingProvince = province

	incomeMap[shipper.OwnerID] += 1

	g.SelectedAreaID, g.SelectedArea2ID, g.OldSelectedAreaID = area.ID, NoArea, old.ID
	fromAID, fromFID := old.ID, FlowID{AreaID: old.ID}
	toFID := FlowID{AreaID: area.ID}
	if old.IsSea() {
		inputFID := FlowID{
			AreaID:   fromAID,
			PID:      shipper.OwnerID,
			Index:    g.SelectedShipper2Index,
			IO:       shipInput,
			Province: province,
		}
		outputFID := FlowID{
			AreaID:   fromAID,
			PID:      shipper.OwnerID,
			Index:    g.SelectedShipper2Index,
			IO:       shipOutput,
			Province: province,
		}
		g.CustomPath = g.CustomPath.addFlow(inputFID, outputFID)
		fromFID = outputFID
	}
	if area.IsSea() {
		toFID = FlowID{
			AreaID:   area.ID,
			PID:      shipper.OwnerID,
			Index:    g.SelectedShipperIndex,
			IO:       shipInput,
			Province: province,
		}
	}
	g.SelectedShipper2Index = g.SelectedShipperIndex
	g.CustomPath = g.CustomPath.addFlow(fromFID, toFID)
	g.SubPhase = OPSelectCityOrShip
	tmpl = "indonesia/select_ship_update"
	return
}

func (g *Game) validateSelectShip(ctx context.Context) (old *Area, area *Area, shipper *Shipper, incomeMap ShipperIncomeMap, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c := g.SelectedCompany()
	shipper = g.SelectedShipper()
	shippingCompany := g.SelectedShippingCompany()
	old, area = g.SelectedArea(), g.SelectedArea2()
	incomeMap = g.ShipperIncomeMap

	switch err = g.validatePlayerAction(ctx); {
	case err != nil:
	case g.Phase != Operations:
		err = sn.NewVError("Expected %q phase, have %q phase.", PhaseNames[Operations], g.PhaseName())
	case !(g.SubPhase == OPSelectShip || g.SubPhase == OPSelectCityOrShip):
		err = sn.NewVError("Expected %q or %q subphase, have %q subphase.",
			SubPhaseNames[OPSelectShip], SubPhaseNames[OPSelectCityOrShip], g.SubPhaseName())
	case c == nil:
		err = sn.NewVError("You must select company to operate.")
	case g.ShipperIncomeMap == nil:
		err = sn.NewVError("Missing temp value for income map.")
	case g.SubPhase == OPSelectShip &&
		(old == nil || area == nil || !c.ZoneFor(old).adjacentToArea(area)):
		err = sn.NewVError("You must select a ship adjacent to the previously selected area.")
	case shipper == nil:
		err = sn.NewVError("You must select a valid ship adjacent to the previously selected area.")
	case shipper.Delivered+1 > shipper.HullSize():
		err = sn.NewVError("The selected ship has already reached its hull limit.")
	case shippingCompany != nil && !(shippingCompany.OwnerID == shipper.OwnerID && shippingCompany.Slot == shipper.Slot):
		err = sn.NewVError("You must select a ship of the same shipping company.")
	}
	return
}

func (g *Game) selectCityOrShip(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch area2 := g.SelectedArea2(); {
	case area2 == nil:
		tmpl = "indonesia/flash_notice"
		err = sn.NewVError("You must select an area having a city or boat.")
	case area2.IsLand():
		tmpl, err = g.selectCity(ctx)
	case area2.IsSea():
		tmpl, err = g.selectShip(ctx)
	default:
		tmpl = "indonesia/flash_notice"
		err = sn.NewVError("Unexpectant value for area received.")
	}
	return
}

func (g *Game) selectCity(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		city                     *City
		company, shippingCompany *Company
		from, to                 Province
		used                     int
	)

	if city, company, from, to, shippingCompany, used, err = g.validateSelectCity(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	city.Delivered[company.Goods()] += 1
	inputFID := toFlowID(g.SelectedAreaID, shippingCompany.OwnerID, g.SelectedShipper2Index, shipInput,
		shippingCompany.Province().Int())
	outputFID := toFlowID(g.SelectedAreaID, shippingCompany.OwnerID, g.SelectedShipper2Index, shipOutput,
		shippingCompany.Province().Int())
	g.CustomPath = g.CustomPath.addFlow(inputFID, outputFID)

	inputFID, outputFID = outputFID, toFlowID(g.SelectedGoodsAreaID)
	g.CustomPath = g.CustomPath.addFlow(inputFID, outputFID)

	inputFID, outputFID = outputFID, targetFID
	g.CustomPath = g.CustomPath.addFlow(inputFID, outputFID)

	// Log
	e := g.newDeliveredGoodEntryFor(g.CurrentPlayer(), company.Goods(), from, to, shippingCompany.OwnerID, used)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	if company.Delivered() == g.RequiredDeliveries {
		tmpl, err = g.receiveIncome(ctx)
	} else {
		g.SubPhase = OPSelectProductionArea
		g.resetShipper()
		tmpl = "indonesia/select_city_update"
	}
	return
}

func (g *Game) resetShipper() {
	g.ShippingCompanyOwnerID, g.ShippingCompanySlot, g.ShipsUsed = NoPlayerID, NoSlot, InvalidUsedShips
}

func (g *Game) validateSelectCity(ctx context.Context) (city *City, c *Company, from Province, to Province, sc *Company, used int, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c, sc = g.SelectedCompany(), g.SelectedShippingCompany()
	a := g.SelectedArea()
	a2 := g.SelectedArea2()
	goodsArea := g.SelectedGoodsArea()

	switch err = g.validatePlayerAction(ctx); {
	case err != nil:
	case g.Phase != Operations:
		err = sn.NewVError("Expected %q phase, have %q phase.", PhaseNames[Operations], g.PhaseName())
	case g.SubPhase != OPSelectCityOrShip:
		err = sn.NewVError("Expected %q subphase, have %q subphase.", SubPhaseNames[OPSelectCityOrShip], g.SubPhaseName())
	case c == nil:
		err = sn.NewVError("You must select company to operate.")
	case goodsArea == nil:
		err = sn.NewVError("Missing selected goods area.")
	case a == nil || a2 == nil || !a2.adjacentToArea(a):
		err = sn.NewVError("You must select a ship adjacent to the previously selected area.")
	case a2.City == nil:
		err = sn.NewVError("You must select an area with a city.")
	case goodsArea.Province() == NoProvince:
		err = sn.NewVError("Invalid 'From' province. Undo turn and try again.")
	case a2.City.Delivered[c.Goods()] >= a2.City.Size:
		err = sn.NewVError("City has already received its allotment of %s.", c.Goods())
	case g.ShipsUsed == InvalidUsedShips:
		err = sn.NewVError("Missing temp value for used ships.")
	case sc == nil:
		err = sn.NewVError("Missing temp value for shipping company owner.")
	default:
		city, from, to, used = a2.City, goodsArea.Province(), a2.Province(), g.ShipsUsed
	}
	return
}

type deliveredGoodEntry struct {
	*Entry
	Goods     Goods
	From      Province
	To        Province
	ShipsUsed int
}

func (g *Game) newDeliveredGoodEntryFor(p *Player, goods Goods, from, to Province, ownerID, used int) (e *deliveredGoodEntry) {
	e = &deliveredGoodEntry{
		Entry:     g.newEntryFor(p),
		Goods:     goods,
		From:      from,
		To:        to,
		ShipsUsed: used,
	}
	e.OtherPlayerID = ownerID
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *deliveredGoodEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("<div>%s delivered %s from the %s province to the city in the %s province using %d ships of %s.</div>", g.NameByPID(e.PlayerID), e.Goods, e.From, e.To, e.ShipsUsed, g.NameByPID(e.OtherPlayerID))
}

func (g *Game) receiveIncome(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		c         *Company
		incomeMap ShipperIncomeMap
	)

	g.SubPhase = OPReceiveIncome
	if c, incomeMap, err = g.validateReceiveIncome(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}
	cp := g.CurrentPlayer()
	otherShips := incomeMap.OtherShips(cp.ID())
	income := c.Delivered()*c.Goods().Price() - (otherShips * 5)
	cp.Rupiah += income
	cp.OpIncome += income
	if otherShips != 0 {
		for pid, count := range incomeMap {
			if pid != cp.ID() {
				income := 5 * count
				p := g.PlayerByID(pid)
				p.Rupiah += income
				p.OpIncome += income
			}
		}
	}

	// Log
	e := g.newReceiveIncomeEntryFor(g.CurrentPlayer(), c.Delivered(), c.Goods(), incomeMap)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	tmpl = g.startCompanyExpansion(ctx)
	return
}

func (g *Game) validateReceiveIncome(ctx context.Context) (c *Company, incomeMap ShipperIncomeMap, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c = g.SelectedCompany()
	incomeMap = g.ShipperIncomeMap
	switch err = g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("Missing selected company.")
	case g.ShipperIncomeMap == nil:
		err = sn.NewVError("Missing income map.")
	}
	return
}

type receiveIncomeEntry struct {
	*Entry
	Delivered     int
	Goods         Goods
	ShipperIncome ShipperIncomeMap
}

func (g *Game) newReceiveIncomeEntryFor(p *Player, delivered int, goods Goods, incomeMap ShipperIncomeMap) (e *receiveIncomeEntry) {
	e = &receiveIncomeEntry{
		Entry:         g.newEntryFor(p),
		Delivered:     delivered,
		Goods:         goods,
		ShipperIncome: incomeMap,
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *receiveIncomeEntry) HTML(ctx context.Context) (s template.HTML) {
	otherShips := e.ShipperIncome.OtherShips(e.PlayerID)
	rupiah := e.Delivered*e.Goods.Price() - (otherShips * 5)
	g := gameFrom(ctx)
	s = restful.HTML("<div>%s received %d rupiah for selling %d %s (%d &times; %d %s - 5 &times; %d ships)</div>",
		g.NameByPID(e.PlayerID), rupiah, e.Delivered, e.Goods, e.Goods.Price(), e.Delivered, e.Goods, otherShips)
	if otherShips != 0 {
		for pid, count := range e.ShipperIncome {
			if pid != e.PlayerID {
				s += restful.HTML("<div>%s received %d rupiah for %d ships used to transport %s.</div>",
					g.NameByPID(pid), 5*count, count, e.Goods)
			}
		}
	}
	return
}

func (g *Game) startCompanyExpansion(ctx context.Context) (tmpl string) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if company := g.SelectedCompany(); company.deliveredAllGoods() {
		g.SubPhase = OPFreeExpansion
		cp := g.CurrentPlayer()
		//g.RequiredExpansions = min(cp.Technologies[ExpansionsTech], len(company.ExpansionAreas()))
		g.RequiredExpansions = company.requiredExpansions()
		if g.RequiredExpansions == 0 {
			company.Operated = true
			cp.PerformedAction = true
			tmpl = "indonesia/completed_expansion_update"
		}
	} else {
		g.SubPhase = OPExpansion
	}
	tmpl = "indonesia/select_city_update"
	return
}

func (g *Game) stopExpanding(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var c *Company

	if c, err = g.validateStopExpanding(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	cp := g.CurrentPlayer()
	cp.PerformedAction = true
	c.Operated = true

	// Log
	e := g.newStopExpandingEntryFor(g.CurrentPlayer())
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	tmpl, act = "indonesia/stop_expanding_update", game.Cache
	return
}

func (g *Game) validateStopExpanding(ctx context.Context) (c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch c, err = g.SelectedCompany(), g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("Missing selected company.")
	case c.IsProductionCompany() && g.SubPhase == OPFreeExpansion:
		err = sn.NewVError("You can not stop expanding.")
	}
	return
}

type stopExpandingEntry struct {
	*Entry
}

func (g *Game) newStopExpandingEntryFor(p *Player) (e *stopExpandingEntry) {
	e = &stopExpandingEntry{Entry: g.newEntryFor(p)}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *stopExpandingEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("<div>%s stopped expanding selected company.</div>", g.NameByPID(e.PlayerID))
}

func (g *Game) expandProduction(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		a *Area
		c *Company
	)

	if a, c, err = g.validateExpandProduction(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	g.Expansions += 1
	a.AddProducer(c)
	c.AddArea(a)
	cp := g.CurrentPlayer()

	// Log
	if g.SubPhase == OPExpansion {
		expense := c.Goods().Price()
		cp.Rupiah -= expense
		cp.OpIncome -= expense
	}
	e := g.newExpandProductionEntryFor(cp, c.Goods(), a.Province(), g.SubPhase == OPFreeExpansion)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	if g.Expansions == g.RequiredExpansions || cp.RemainingExpansions() == 0 {
		c.Operated = true
		cp.PerformedAction = true
		tmpl = "indonesia/completed_expansion_update"
	} else {
		tmpl = "indonesia/company_expansion_dialog"
	}
	return
}

func (p *Player) RemainingExpansions() int {
	if p.Game().SubPhase == OPFreeExpansion {
		return p.Game().RequiredExpansions - p.Game().Expansions
	}
	return p.Technologies[ExpansionsTech] - p.Game().Expansions
}

func (g *Game) validateExpandProduction(ctx context.Context) (a *Area, c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	cp := g.CurrentPlayer()
	switch a, c, err = g.SelectedArea(), g.SelectedCompany(), g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("Missing selected company.")
	case a == nil:
		err = sn.NewVError("Missing selected area.")
	case g.SubPhase == OPExpansion && cp.Rupiah < c.Goods().Price():
		err = sn.NewVError("You do not have %d rupiah to pay for expansion.", c.Goods().Price())
	case !c.ExpansionAreas().include(a):
		err = sn.NewVError("Selected area is not a valid expansion area.")
	case cp.RemainingExpansions() == 0:
		err = sn.NewVError("You have already performed the allotted number of expansions.")
	}
	return
}

type expandProductionEntry struct {
	*Entry
	Goods    Goods
	Province Province
	Paid     int
}

func (g *Game) newExpandProductionEntryFor(p *Player, goods Goods, province Province, free bool) (e *expandProductionEntry) {
	e = &expandProductionEntry{
		Entry:    g.newEntryFor(p),
		Goods:    goods,
		Province: province,
	}
	if !free {
		e.Paid = goods.Price()
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *expandProductionEntry) HTML(ctx context.Context) (s template.HTML) {
	g := gameFrom(ctx)
	n := g.NameByPID(e.PlayerID)
	if e.Paid == 0 {
		s = restful.HTML("<div>%s freely expanded the selected %s company to an area in the %s province.</div>", n, e.Goods, e.Province)
	} else {
		s = restful.HTML("<div>%s paid %d to expand the selected %s company to an area in the %s province.</div>", n, e.Paid, e.Goods, e.Province)
	}
	return
}

func (g *Game) expandShipping(ctx context.Context) (tmpl string, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var (
		a *Area
		c *Company
	)

	if a, c, err = g.validateExpandShipping(ctx); err != nil {
		tmpl = "indonesia/flash_notice"
		return
	}

	g.Expansions += 1
	c.Operated = true
	c.AddShipIn(a)
	cp := g.CurrentPlayer()

	// Log
	e := g.newExpandShippingEntryFor(g.CurrentPlayer(), c, a)
	restful.AddNoticef(ctx, string(e.HTML(ctx)))
	if g.Expansions < cp.Technologies[ExpansionsTech] && c.Ships() < c.MaxShips() {
		tmpl = "indonesia/select_shipping_area_update"
	} else {
		cp.PerformedAction = true
		tmpl = "indonesia/completed_expansion_update"
	}
	return
}

func (g *Game) validateExpandShipping(ctx context.Context) (a *Area, c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	a = g.SelectedArea()
	c = g.SelectedCompany()
	maxShips := c.MaxShips()
	cp := g.CurrentPlayer()

	switch err = g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("Missing selected company.")
	case a == nil:
		err = sn.NewVError("Missing selected area.")
	case !g.freeShippingExpansionAreas().include(a):
		err = sn.NewVError("Selected area is not a valid expansion area.")
	case g.Expansions >= cp.Technologies[ExpansionsTech]:
		err = sn.NewVError("You have already performed the allotted number of expansion.")
	case c.Ships() >= maxShips:
		err = sn.NewVError("The selected company is already at it's ship limit of %d for the era.", maxShips)
	case c.MaxShips() == c.Ships():
		err = sn.NewVError("The selected shipping company has already expanded to its ship limit for the era.")
	}
	return
}

type expandShippingEntry struct {
	*Entry
	Company Company
	Area    Area
}

func (g *Game) newExpandShippingEntryFor(p *Player, company *Company, area *Area) (e *expandShippingEntry) {
	e = &expandShippingEntry{
		Entry:   g.newEntryFor(p),
		Company: *company,
		Area:    *area,
	}
	p.Log = append(p.Log, e)
	g.Log = append(g.Log, e)
	return
}

func (e *expandShippingEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	return restful.HTML("<div>%s freely expanded the %s company to a sea area near the %s province.</div>",
		g.NameByPID(e.PlayerID), e.Company.String(), e.Area.Province().String())
}

func (g *Game) resetShipping() {
	for _, a := range g.seaAreas() {
		a.Used = false
		for _, s := range a.Shippers {
			s.Delivered = 0
		}
	}
}

func (p *Player) CanFreeExpansion() bool {
	g := p.Game()
	return g.Phase == Operations && g.SubPhase == OPFreeExpansion
}

func (g *Game) acceptProposedFlow(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var c *Company
	if c, err = g.validateAcceptProposedFlow(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}
	c.Operated = true

	g.ShipperIncomeMap = g.ProposedShips(g.ProposedPath)
	for aid, v := range g.ProposedCities() {
		g.GetArea(aid).City.Delivered[c.Goods()] += v
	}
	for fid, v := range g.ProposedPath[sourceFID] {
		count := 0
		for _, a := range c.ZoneFor(g.GetArea(fid.AreaID)).Areas() {
			a.Used = true
			if count += 1; count == v {
				break
			}
		}
	}

	if tmpl, err = g.receiveIncome(ctx); err == nil {
		act = game.Cache
	} else {
		act = game.None
	}
	return
}

func (g *Game) validateAcceptProposedFlow(ctx context.Context) (c *Company, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch c, err = g.SelectedCompany(), g.validatePlayerAction(ctx); {
	case err != nil:
	case c == nil:
		err = sn.NewVError("Missing selected company.")
	case c.Delivered() != 0:
		err = sn.NewVError("You can not accept proposed deliveries.")
	}
	return
}
