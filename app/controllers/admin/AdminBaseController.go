package admin

import (
	//	"github.com/revel/revel"
	//	"go.mongodb.org/mongo-driver/v2/bson"
	//	"encoding/json"
	"github.com/yangphere/leanote/app/controllers"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	//	"io/ioutil"
	//	"fmt"
	//	"math"
	//	"strconv"
	"strings"
)

// 公用Controller, 其它Controller继承它
type AdminBaseController struct {
	controllers.BaseController // 不能用*BaseController
}

// 得到sorterField 和 isAsc
// okSorter = ['email', 'username']
func (c AdminBaseController) getSorter(sorterField string, isAsc bool, okSorter []string) (string, bool) {
	sorter := ""
	c.Params.Bind(&sorter, "sorter")
	if sorter == "" {
		return sorterField, isAsc
	}

	// sorter形式 email-up, email-down
	s2 := strings.Split(sorter, "-")
	if len(s2) != 2 {
		return sorterField, isAsc
	}

	// 必须是可用的sorter
	if okSorter != nil && len(okSorter) > 0 {
		if !InArray(okSorter, s2[0]) {
			return sorterField, isAsc
		}
	}

	sorterField = strings.Title(s2[0])
	if s2[1] == "up" {
		isAsc = true
	} else {
		isAsc = false
	}
	c.ViewArgs["sorter"] = sorter
	return sorterField, isAsc
}

func (c AdminBaseController) updateConfig(keys []string) service.ConfigMutationResult {
	userId := c.GetUserId()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = c.Params.Values.Get(key)
	}
	return configService.UpdateGlobalStringConfigs(userId, values)
}

func (c AdminBaseController) requireAdmin() error {
	if configService == nil {
		return service.ErrAdminPrincipal
	}
	return configService.RequireAdmin(c.RequestContext(), c.GetUserId(), c.GetUsername())
}
