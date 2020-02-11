package indonesia

import (
	"encoding/gob"
	"fmt"
	"html/template"

	"bitbucket.org/SlothNinja/slothninja-games/sn/contest"
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/restful"
	"bitbucket.org/SlothNinja/slothninja-games/sn/send"
	"go.chromium.org/gae/service/mail"
	"golang.org/x/net/context"
)

func init() {
	gob.Register(new(endGameEntry))
	gob.Register(new(announceWinnersEntry))
	//gob.Register(new(doubleIncomeEntry))
	gob.Register(new(doubleFinalIncomeEntry))
}

func (g *Game) endGame(ctx context.Context) (cs contest.Contests) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")
	g.Phase = EndGame

	g.doubleFinalIncome()

	places := g.determinePlaces(ctx)
	g.SetWinners(places[0])
	cs = contest.GenContests(ctx, places)
	g.newEndGameEntry()
	return
}

func (g *Game) doubleFinalIncome() {
	income := make(finalIncomeMap, 0)
	// Double Final Income
	for _, p := range g.Players() {
		income[p.ID()] = &finalIncome{
			Before: p.Score(),
			Income: p.OpIncome,
			After:  p.Score() + p.OpIncome,
		}
		p.Rupiah += p.OpIncome
	}
	g.newDoubleFinalIncomeEntry(income)
}

func toIDS(places []Players) [][]int64 {
	sids := make([][]int64, len(places))
	for i, players := range places {
		for _, p := range players {
			sids[i] = append(sids[i], p.User().ID)
		}
	}
	return sids
}

type endGameEntry struct {
	*Entry
}

func (g *Game) newEndGameEntry() {
	e := &endGameEntry{
		Entry: g.newEntry(),
	}
	g.Log = append(g.Log, e)
}

func (e *endGameEntry) HTML(ctx context.Context) template.HTML {
	return restful.HTML("")
}

func (g *Game) SetWinners(rmap contest.ResultsMap) {
	g.Phase = AnnounceWinners
	g.Status = game.Completed

	g.setCurrentPlayers()
	g.WinnerIDS = nil
	for key := range rmap {
		p := g.PlayerByUserID(key.IntID())
		g.WinnerIDS = append(g.WinnerIDS, p.ID())
	}

	g.newAnnounceWinnersEntry()
}

func (g *Game) SendEndGameNotifications(ctx context.Context) error {
	g.Phase = GameOver
	g.Status = game.Completed

	ms := make([]*mail.Message, len(g.Players()))
	sender := "webmaster@slothninja.com"
	subject := fmt.Sprintf("SlothNinja Games: Indonesia #%d Has Ended", g.ID)
	var body string
	for _, p := range g.Players() {
		body += fmt.Sprintf("%s scored %d points.\n", g.NameFor(p), p.Score())
	}

	var names []string
	for _, p := range g.Winners() {
		names = append(names, g.NameFor(p))
	}
	body += fmt.Sprintf("\nCongratulations to: %s.", restful.ToSentence(names))

	for i, p := range g.Players() {
		ms[i] = &mail.Message{
			To:      []string{p.User().Email},
			Sender:  sender,
			Subject: subject,
			Body:    body,
		}
	}
	return send.Message(ctx, ms...)
}

type announceWinnersEntry struct {
	*Entry
}

func (g *Game) newAnnounceWinnersEntry() *announceWinnersEntry {
	e := &announceWinnersEntry{
		Entry: g.newEntry(),
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *announceWinnersEntry) HTML(ctx context.Context) template.HTML {
	g := gameFrom(ctx)
	names := make([]string, len(g.Winners()))
	for i, winner := range g.Winners() {
		names[i] = g.NameFor(winner)
	}
	return restful.HTML("Congratulations to: %s.", restful.ToSentence(names))
}

func (g *Game) Winners() Players {
	length := len(g.WinnerIDS)
	if length == 0 {
		return nil
	}
	ps := make(Players, length)
	for i, pid := range g.WinnerIDS {
		ps[i] = g.PlayerByID(pid)
	}
	return ps
}

//type doubleIncomeEntry struct {
//	*Entry
//	Income int
//}
//
//func (g *Game) newDoubleIncomeEntryFor(p *Player) (e *doubleIncomeEntry) {
//	e = &doubleIncomeEntry{
//		Entry:  g.newEntryFor(p),
//		Income: p.OpIncome,
//	}
//	p.Log = append(p.Log, e)
//	g.Log = append(g.Log, e)
//	return
//}
//
//func (e *doubleIncomeEntry) HTML(ctx context.Context) template.HTML {
//	g := gameFrom(ctx)
//	return restful.HTML("<div>%s received operations income of Rp %d, which was doubled.</div>",
//		g.NameByPID(e.PlayerID), e.Income)
//}

type finalIncome struct {
	Before int
	Income int
	After  int
}

type finalIncomeMap map[int]*finalIncome

type doubleFinalIncomeEntry struct {
	*Entry
	FinalIncome finalIncomeMap
}

func (g *Game) newDoubleFinalIncomeEntry(f finalIncomeMap) *doubleFinalIncomeEntry {
	e := &doubleFinalIncomeEntry{
		Entry:       g.newEntry(),
		FinalIncome: f,
	}
	g.Log = append(g.Log, e)
	return e
}

func (e *doubleFinalIncomeEntry) HTML(ctx context.Context) (s template.HTML) {
	g := gameFrom(ctx)
	s = restful.HTML("<div>Final operations income doubled as follows:</div>")
	s += restful.HTML("<div><table class='strippedDataTable'><thead><tr><th>Player</th><th>Score</th><th>Income</th><th>Final</th></tr></thead><tbody>")
	for pid, income := range e.FinalIncome {
		p := g.PlayerByID(pid)
		s += restful.HTML("<tr><td>%s</td><td>%d</td><td>%d</td><td>%d</td></tr>",
			g.NameFor(p), income.Before, income.Income, income.After)
	}
	s += restful.HTML("</tbody></table></div>")
	return
}
