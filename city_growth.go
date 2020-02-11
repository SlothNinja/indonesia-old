package indonesia

import (
	"encoding/gob"
	"fmt"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"golang.org/x/net/context"
)

const (
	Size1 int = iota
	Size2
	Size3
)

func init() {
	gob.Register(new(cityGrowthEntry))
	gob.Register(new(deliveredGoodsEntry))
}

type cityGrowthMap map[int]Cities

func (g *Game) startCityGrowth(ctx context.Context) (cs contest.Contests) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	g.Phase = CityGrowth
	g.setCurrentPlayers(g.Players()[0])
	g.newDeliveredGoodsEntry()
	cmap := g.CityGrowthMap()

	switch c2growth, c2stonesToUse, c3growth, c3stonesToUse := len(cmap[Size2]), g.C2StonesToUse(cmap),
		len(cmap[Size3]), g.C3StonesToUse(cmap); {
	case c3stonesToUse > 0 && c3stonesToUse < c3growth:
	case c2stonesToUse > 0 && c2stonesToUse < c2growth:
	default:
		for _, cities := range cmap {
			for _, city := range cities {
				g.grow(city)
			}
		}
		cs = g.startNewEra(ctx)
	}
	return
}

func (g *Game) grow(c *City) {
	size := c.Size
	c.Size += 1
	c.Grew = true
	g.CityStones[size-1] += 1
	g.CityStones[size] -= 1
	g.newCityGrowthEntry(c.Province(), c.Size)
}

func (g *Game) CityGrowthMap() (cmap cityGrowthMap) {
	cmap = make(cityGrowthMap, 0)
	for _, c := range g.Cities() {
		if c.CanGrow() {
			cmap[c.Size] = append(cmap[c.Size], c)
		}
	}
	return
}

func (g *Game) C3StonesToUse(cmap cityGrowthMap) int {
	return min(g.CityStones[Size3], len(cmap[2]))
}

func (g *Game) C2StonesToUse(cmap cityGrowthMap) int {
	return min(g.CityStones[Size2]+g.C3StonesToUse(cmap), len(cmap[1]))
}

type cityGrowthEntry struct {
	*Entry
	Province Province
	Size     int
}

func (g *Game) newCityGrowthEntry(p Province, s int) (e *cityGrowthEntry) {
	e = &cityGrowthEntry{
		Entry:    g.newEntry(),
		Province: p,
		Size:     s,
	}
	g.Log = append(g.Log, e)
	return
}

func (e *cityGrowthEntry) HTML(ctx context.Context) template.HTML {
	return restful.HTML("<div>The city in %s grew to a size %d city.</div>", e.Province, e.Size)
}

type city struct {
	Size      int
	Delivered []int
}

type deliveredGoodsMap map[AreaID]*city

type deliveredGoodsEntry struct {
	*Entry
	Delivered     deliveredGoodsMap
	ProducedGoods []bool
}

func (g *Game) newDeliveredGoodsEntry() *deliveredGoodsEntry {
	dgm := make(deliveredGoodsMap, 0)
	for _, c := range g.Cities() {
		dgm[c.a.ID] = &city{
			Size:      c.Size,
			Delivered: make([]int, len(c.Delivered)),
		}
		copy(dgm[c.a.ID].Delivered, c.Delivered)
	}
	e := &deliveredGoodsEntry{
		Entry:         g.newEntry(),
		Delivered:     dgm,
		ProducedGoods: g.ProducedGoods(),
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *deliveredGoodsEntry) HTML(ctx context.Context) (s template.HTML) {
	g := gameFrom(ctx)
	var goods []string
	for i, produced := range e.ProducedGoods {
		if produced {
			goods = append(goods, g.ToGoods(i).String())
		}
	}
	s = restful.HTML("<div>Goods produced: %s.</div>", restful.ToSentence(goods))
	s += restful.HTML("<div>Cities received goods as follows:</div>")
	s += restful.HTML("<div><table class='strippedDataTable'><thead><tr><th>City In</th><th>Size</th>")
	for _, good := range goods {
		s += restful.HTML("<th>%s</th>", good)
	}
	s += restful.HTML("</tr></thead><tbody>")
	for aid, city := range e.Delivered {
		s += restful.HTML("<tr><td>%s</td><td>%d</td>", g.GetArea(aid).Province(), city.Size)
		for i, produced := range e.ProducedGoods {
			if produced {
				s += restful.HTML("<td>%d</td>", city.Delivered[g.ToGoods(i)])
			}
		}
		s += restful.HTML("</tr>")
	}
	s += restful.HTML("</tbody></table></div>")
	return s
}

func (g *Game) cityGrowth(ctx context.Context) (tmpl string, act game.ActionType, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	var cs Cities

	if cs, err = g.validateCityGrowth(ctx); err != nil {
		tmpl, act = "indonesia/flash_notice", game.None
		return
	}

	for _, c := range cs {
		g.grow(c)
	}
	g.CurrentPlayer().PerformedAction = true
	tmpl, act = "indonesia/city_growth_update", game.Cache
	return
}

func (g *Game) validateCityGrowth(ctx context.Context) (cs Cities, err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	c := restful.GinFrom(ctx)

	cmap := g.CityGrowthMap()
	for size, cities := range cmap {
		var count, stonesToUse int
		if size == 1 {
			count, stonesToUse = 0, g.C2StonesToUse(cmap)
		} else if size == 2 {
			count, stonesToUse = 0, g.C3StonesToUse(cmap)
		}

		for i, city := range cities {
			key := fmt.Sprintf("%d-%d", size, i)
			if v := c.PostForm(key); v == "on" {
				count += 1
				cs = append(cs, city)
			}
		}
		switch {
		case count < stonesToUse:
			err = sn.NewVError("You did not select enough cities.  You selected %d size %d cities, but need to select %d size %d cities.", count, size+1, stonesToUse, size+1)
		case count > stonesToUse:
			err = sn.NewVError("You selected too many cities.  You selected %d size %d cities, but need to select %d size %d cities.", count, size+1, stonesToUse, size+1)
		}
	}
	return
}
