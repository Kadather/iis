package view

import (
	"fmt"
	"net/http"

	"github.com/coyove/iis/common"
	"github.com/coyove/iis/dal"
	"github.com/coyove/iis/ik"
	"github.com/coyove/iis/middleware"
	"github.com/coyove/iis/model"
	"github.com/gin-gonic/gin"
)

func User(g *gin.Context) {
	m, _ := g.Cookie("mode")
	p := struct {
		UUID         string
		Challenge    string
		Survey       interface{}
		SearchSurvey string
		User         *model.User
		SiteKey      string
		DarkCaptcha  bool
		OTT          string
		OTTUsername  string
		OTTEmail     string
	}{
		Survey:      middleware.Survey,
		SiteKey:     common.Cfg.HCaptchaSiteKey,
		DarkCaptcha: m == "dark",
		OTT:         g.Query("ott"),
		OTTUsername: g.Query("ott-username"),
		OTTEmail:    g.Query("ott-email"),
	}

	p.UUID, p.Challenge = ik.MakeToken(g)
	p.User = getUser(g)
	if p.User != nil {
		p.User.SetShowList('S')
		if p.User.IsAdmin() {
			p.SearchSurvey = model.SearchMetrics()
		}
	}
	g.HTML(200, "user.html", p)
}

func UserList(g *gin.Context) {
	p := struct {
		UUID     string
		List     []dal.FollowingState
		EError   string
		Next     string
		ListType string
		You      *model.User
		User     *model.User
	}{
		UUID:     ik.MakeUUID(g, nil),
		EError:   g.Query("error"),
		ListType: g.Param("type"),
	}

	p.You = getUser(g)
	if p.You == nil {
		redirectVisitor(g)
		return
	}

	p.User, _ = dal.GetUser(g.Param("uid"))
	if p.User == nil {
		p.User = p.You
	}

	p.User.Buildup(p.You)

	switch p.ListType {
	case "blacklist":
		if p.User != p.You {
			g.Redirect(302, "/user/blacklist")
			return
		}
		p.List, p.Next = dal.GetRelationList(p.User, ik.NewID(ik.IDBlacklist, p.User.ID), g.Query("n"), int(common.Cfg.PostsPerPage))
		p.User.SetShowList('b')
	case "followers":
		p.List, p.Next = dal.GetRelationList(p.User, ik.NewID(ik.IDFollower, p.User.ID), g.Query("n"), int(common.Cfg.PostsPerPage))
		p.User.SetShowList('s')
	case "twohops":
		if p.You.ID == p.User.ID {
			g.Redirect(302, "/user/followings")
			return
		}
		p.List, p.Next = dal.GetCommonFollowingList(p.You.ID, p.User.ID, g.Query("n"), int(common.Cfg.PostsPerPage))
		p.User.SetShowList('r')
	default:
		p.List, p.Next = dal.GetFollowingList(ik.NewID(ik.IDFollowing, p.User.ID), g.Query("n"), int(common.Cfg.PostsPerPage), true)
		p.User.SetShowList('f')
	}

	g.HTML(200, "user_list.html", p)
}

// var ig, _ = identicon.New("github", 5, 3)

func Avatar(g *gin.Context) {
	id := g.Param("id")
	if len(id) == 0 {
		g.Status(404)
		return
	}

	hash := (model.User{ID: id}).IDHash()
	path := fmt.Sprintf("tmp/images/%016x@%s", hash, id)

	// 	if g.Query("q") != "0" {
	if common.Cfg.S3Region != "" {
		path := fmt.Sprintf("%s/%016x@%s?q=%s", common.Cfg.MediaDomain, hash, id, g.Query("q"))
		g.Redirect(302, path)
	} else {
		http.ServeFile(g.Writer, g.Request, path)
	}
	// 	} else {
	// 		ii, err := ig.Draw("iis" + id)
	// 		if err != nil {
	// 			log.Println(err)
	// 			g.Status(404)
	// 			return
	// 		}
	// 		g.Writer.Header().Add("Content-Type", "image/jpeg")
	// 		g.Writer.Header().Add("Cache-Control", "public")
	// 		ii.Jpeg(100, 80, g.Writer)
	// 	}
}

func UserLikes(g *gin.Context) {
	p := ArticlesTimelineView{
		IsUserLikeTimeline: true,
		MediaOnly:          g.Query("media") != "",
		ReplyView:          makeReplyView(g, ""),
		You:                getUser(g),
	}

	if p.You == nil {
		redirectVisitor(g)
		return
	}

	if uid := g.Param("uid"); uid != "master" {
		p.User, _ = dal.GetUser(uid)
		if p.User == nil {
			p.User = p.You
		}
	} else {
		p.User = p.You
	}

	var cursor string
	if pa, _ := dal.GetArticle(ik.NewID(ik.IDLike, p.User.ID).String()); pa != nil {
		cursor = pa.PickNextID(p.MediaOnly)
	}

	a, next := dal.WalkLikes(p.MediaOnly, int(common.Cfg.PostsPerPage), cursor)
	fromMultiple(&p.Articles, a, 0, getUser(g))
	p.Next = next

	g.HTML(200, "timeline.html", p)
}

func APIGetUserInfoBox(g *gin.Context) {
	you := getUser(g)

	if you == nil {
		g.Status(400)
		return
	}

	u, _ := dal.GetUserWithSettings(g.Param("id"))
	if u == nil {
		g.String(200, "internal/error")
		return
	}

	if you != nil {
		u.Buildup(you)
	}

	s := middleware.RenderTemplateString("user_public.html", u)
	g.String(200, "ok:"+s)
}
