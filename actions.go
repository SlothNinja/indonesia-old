package indonesia

import (
	"bitbucket.org/SlothNinja/slothninja-games/sn"
	"bitbucket.org/SlothNinja/slothninja-games/sn/log"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user"
	"golang.org/x/net/context"
)

func (g *Game) validatePlayerAction(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	switch cp := g.CurrentPlayer(); {
	case cp.PerformedAction:
		err = sn.NewVError("You have already performed an action.")
	case !g.CUserIsCPlayerOrAdmin(ctx):
		err = sn.NewVError("Only the current player can perform an action.")
	}
	return
}

func (g *Game) validateAdminAction(ctx context.Context) (err error) {
	log.Debugf(ctx, "Entering")
	defer log.Debugf(ctx, "Exiting")

	if !user.IsAdmin(ctx) {
		err = sn.NewVError("Only an admin can perform the selected action.")
	}
	return
}

//type MultiActionID int
//
//const (
//	noMultiAction MultiActionID = iota
//	startedEmpireMA
//	boughtArmiesMA
//	equippedArmyMA
//	placedArmiesMA
//	usedScribeMA
//	selectedWorkerMA
//	placedWorkerMA
//	tradedResourceMA
//	expandEmpireMA
//	builtCityMA
//)
