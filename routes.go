package indonesia

import (
	"bitbucket.org/SlothNinja/slothninja-games/sn/game"
	"bitbucket.org/SlothNinja/slothninja-games/sn/mlog"
	"bitbucket.org/SlothNinja/slothninja-games/sn/type"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user"
	"bitbucket.org/SlothNinja/slothninja-games/sn/user/stats"
	"github.com/gin-gonic/gin"
)

func AddRoutes(prefix string, engine *gin.Engine) {
	// New
	g1 := engine.Group(prefix)
	g1.GET("/game/new",
		user.RequireCurrentUser(),
		gType.SetTypes(),
		NewAction(prefix),
	)

	// Create
	g1.POST("/game",
		user.RequireCurrentUser(),
		Create(prefix),
	)

	// Show
	g1.GET("/game/show/:hid",
		//game.FetchHeader(GamesRoot),
		Fetch,
		mlog.Get,
		game.SetAdmin(false),
		Show(prefix),
	)

	// Admin
	g1.GET("/game/admin/:hid",
		//game.FetchHeader(GamesRoot),
		Fetch,
		mlog.Get,
		game.SetAdmin(true),
		Show(prefix),
	)

	// Undo
	g1.POST("/game/undo/:hid",
		//game.FetchHeader(GamesRoot),
		//UndoUpdate(),
		Fetch,
		Undo(prefix),
	)

	//	// Redo
	//	g1.POST("/game/redo/:hid",
	//		//game.FetchHeader(GamesRoot),
	//		RedoUpdate(),
	//		Redo(prefix),
	//	)
	//
	//	// Reset
	//	g1.POST("/game/reset/:hid",
	//		//game.FetchHeader(GamesRoot),
	//		ResetUpdate(),
	//		Reset(prefix),
	//	)

	// Finish
	g1.POST("/game/finish/:hid",
		//game.FetchHeader(GamesRoot),
		Fetch,
		stats.Fetch(user.CurrentFrom),
		Finish(prefix),
	)

	// Drop
	g1.POST("/game/drop/:hid",
		user.RequireCurrentUser(),
		//game.FetchHeader(GamesRoot),
		Fetch,
		Drop(prefix),
	)

	// Accept
	g1.POST("/game/accept/:hid",
		user.RequireCurrentUser(),
		//game.FetchHeader(GamesRoot),
		Fetch,
		Accept(prefix),
	)

	// Update
	g1.PUT("/game/show/:hid",
		user.RequireCurrentUser(),
		//game.FetchHeader(GamesRoot),
		Fetch,
		game.RequireCurrentPlayerOrAdmin(),
		game.SetAdmin(false),
		Update(prefix),
	)

	// Admin Update
	g1.POST("/game/admin/:hid",
		user.RequireCurrentUser(),
		//game.FetchHeader(GamesRoot),
		Fetch,
		game.RequireCurrentPlayerOrAdmin(),
		game.SetAdmin(true),
		Update(prefix),
	)

	g1.PUT("/game/admin/:hid",
		user.RequireCurrentUser(),
		//game.FetchHeader(GamesRoot),
		Fetch,
		game.RequireCurrentPlayerOrAdmin(),
		game.SetAdmin(true),
		Update(prefix),
	)

	// Index
	g1.GET("/games/:status",
		gType.SetTypes(),
		Index(prefix),
	)

	// JSON Data for Index
	g1.POST("games/:status/json",
		gType.SetTypes(),
		game.GetFiltered(gType.Indonesia),
		JSONIndexAction(prefix),
	)

	// Add Message
	g1.PUT("/game/show/:hid/addmessage",
		user.RequireCurrentUser(),
		mlog.Get,
		mlog.AddMessage(prefix),
	)
}
