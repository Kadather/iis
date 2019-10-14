package view

import (
	"log"

	"github.com/coyove/iis/cmd/ch/config"
	"github.com/coyove/iis/cmd/ch/ident"
	"github.com/gin-gonic/gin"
)

func New(g *gin.Context) {
	ident.DecryptQuery(g)
	var pl = struct {
		UUID      string
		Reply     string
		Abstract  string
		Challenge string
		Tags      []string
		IsAdmin   bool

		RTitle, RAuthor, RContent, RCat, EError string
	}{
		RTitle:   g.Query("title"),
		RContent: g.Query("content"),
		RCat:     g.Query("cat"),
		RAuthor:  g.Query("author"),
		EError:   g.Query("error"),
		Tags:     config.Cfg.Tags,
		IsAdmin:  ident.IsAdmin(g),
	}

	var answer [4]byte
	pl.UUID, answer = ident.MakeToken(g)
	pl.Challenge = ident.GenerateCaptcha(answer)

	if pl.RAuthor == "" {
		pl.RAuthor, _ = g.Cookie("id")
	}

	g.HTML(200, "newpost.html", pl)
}

func Edit(g *gin.Context) {
	var pl = struct {
		UUID           string
		Reply          string
		Tags           []string
		RAuthor        string
		IsAdmin        bool
		IsAuthorBanned bool
		Article        ArticleView
	}{
		Reply: g.Param("id"),
		Tags:  config.Cfg.Tags,
	}

	pl.UUID, _ = ident.MakeToken(g)
	pl.RAuthor, _ = g.Cookie("id")
	pl.IsAdmin = ident.IsAdmin(pl.RAuthor)

	a, err := m.Get(ident.StringBytes(g, pl.Reply))
	if err != nil {
		log.Println(err)
		g.Redirect(302, "/cat")
		return
	}

	pl.Article.from(a, _showcontent, g)
	pl.IsAuthorBanned = m.IsBanned(nil, a.Author)

	g.HTML(200, "editpost.html", pl)
}