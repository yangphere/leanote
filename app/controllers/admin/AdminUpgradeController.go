package admin

import (
	"github.com/revel/revel"
	//	"encoding/json"
	"github.com/yangphere/leanote/app/info"
	//	"io/ioutil"
)

// Upgrade controller
type AdminUpgrade struct {
	AdminBaseController
}

func (c AdminUpgrade) UpgradeBlog() revel.Result {
	re := info.NewRe()
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = upgradeService.UpgradeBlog()
	return c.RenderJSON(re)
}

func (c AdminUpgrade) UpgradeBetaToBeta2() revel.Result {
	re := info.NewRe()
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = upgradeService.UpgradeBetaToBeta2(c.GetUserId())
	return c.RenderJSON(re)
}

func (c AdminUpgrade) UpgradeBeta3ToBeta4() revel.Result {
	re := info.NewRe()
	if err := c.requireAdmin(); err != nil {
		re.Msg = "admin.forbidden"
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = upgradeService.Api(c.GetUserId())
	return c.RenderJSON(re)
}
