package main

import (
	"encoding/json"
	"github.com/Yuukirn/egs"
	"github.com/Yuukirn/egs/router"
	"github.com/Yuukirn/egs/security"
	"github.com/gin-gonic/gin"
	"net/http"
)

var ping = router.NewRouterX(func(c *gin.Context) {
	c.String(http.StatusOK, "pong")
})

type TestStruct struct {
	ID    string `uri:"id" validate:"required" json:"id"`
	Name  string `form:"name" json:"name"`
	Age   int    `form:"age" json:"age"`
	Token string `header:"authorization" validate:"required" json:"token"`
}

type TestResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func TestApi(c *gin.Context, req TestStruct) {
	bytes, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}
	println(string(bytes))
	c.JSON(http.StatusOK, TestResp{
		Code: 200,
		Msg:  "OK",
	})
}

var jwtAuth = &security.Bearer{
	AuthName: "jwtAuth",
}

var test = router.NewRouter(TestApi,
	router.Tags("test"),
	router.Summary("test summary"),
	router.Desc("test desc"),
	router.Req(router.Request{
		Model: &TestStruct{},
	}),
	router.Resp(router.Response{
		"200": router.ResponseItem{
			Model: &TestResp{},
		},
	}),
)

func testMiddleware(c *gin.Context) {
	header := c.GetHeader("authorization")
	if header == "" {
		c.Abort()
	}
	c.Next()
}

func main() {
	app := egs.New(egs.NewSwagger("example", "", "3.0.0"))
	app.GET("/ping", ping)

	testGroup := app.Group("test", egs.Handlers(testMiddleware), egs.Security(jwtAuth))
	{
		testGroup.POST("/:id", test)
	}

	app.Run(":8080")
}
