package controllers

import (
	"github.com/revel/revel"
	//	"encoding/json"
	//	"go.mongodb.org/mongo-driver/v2/bson"
	"github.com/yangphere/leanote/app/info"
	"github.com/yangphere/leanote/app/lea/captcha"
	//	"github.com/yangphere/leanote/app/types"
	//	"io/ioutil"
	//	"fmt"
	//	"math"
	//	"os"
	//	"path"
	//	"strconv"
	"io"
	"net/http"
)

// 验证码服务
type Captcha struct {
	BaseController
}

type Ca string

func (r Ca) Apply(req *revel.Request, resp *revel.Response) {
	resp.WriteHeader(http.StatusOK, "image/png")
}

func (c Captcha) Get() revel.Result {
	sessionId, err := c.AnonymousSessionID()
	if err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	image, str := captcha.Fetch()
	if err := sessionService.SetCaptcha(sessionId, str); err != nil {
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}
	c.Response.ContentType = "image/png"
	out := io.Writer(c.Response.GetWriter())
	if _, err := image.WriteTo(out); err != nil {
		c.Response.ContentType = ""
		return c.RenderJSON(info.Re{Ok: false, Msg: "storage"})
	}

	//	LogJ(c.Session)
	//	Log("------")
	//	Log(str)
	//	Log(sessionId)

	return Ca("")
}
