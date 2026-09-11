package api

import (
	"errors"
	"github.com/revel/revel"
	//	"encoding/json"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"github.com/yangphere/leanote/app/service"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
	//	"github.com/yangphere/leanote/app/types"
	"io/ioutil"
	//	"fmt"
	//	"math"
	"os"
	//	"path"
	//	"strconv"
)

type ApiUser struct {
	ApiBaseContrller
}

// 获取用户信息
// [OK]
func (c ApiUser) Info() revel.Result {
	re := info.NewApiRe()

	userInfo := c.getUserInfo()
	if userInfo.UserId.IsZero() {
		return c.RenderJSON(re)
	}
	apiUser := info.ApiUser{
		UserId:   userInfo.UserId.Hex(),
		Username: userInfo.Username,
		Email:    userInfo.Email,
		Logo:     userInfo.Logo,
		Verified: userInfo.Verified,
	}
	return c.RenderJSON(apiUser)
}

// 修改用户名
// [OK]
func (c ApiUser) UpdateUsername(username string) revel.Result {
	re := info.NewApiRe()
	isDemo, err := c.isDemoUser()
	if err != nil {
		re.Msg = apiDemoPolicyErrorMessage(err)
		return c.RenderJSON(re)
	}
	if isDemo {
		re.Msg = "cannotUpdateDemo"
		return c.RenderJSON(re)
	}

	if re.Ok, re.Msg = Vd("username", username); !re.Ok {
		return c.RenderJSON(re)
	}

	re.Ok, re.Msg = userService.UpdateUsername(c.getUserId(), username)
	return c.RenderJSON(re)
}

// 修改密码
// [OK]
func (c ApiUser) UpdatePwd(oldPwd, pwd string) revel.Result {
	re := info.NewApiRe()
	isDemo, err := c.isDemoUser()
	if err != nil {
		re.Msg = apiDemoPolicyErrorMessage(err)
		return c.RenderJSON(re)
	}
	if isDemo {
		re.Msg = "cannotUpdateDemo"
		return c.RenderJSON(re)
	}
	if re.Ok, re.Msg = Vd("password", oldPwd); !re.Ok {
		return c.RenderJSON(re)
	}
	if re.Ok, re.Msg = Vd("password", pwd); !re.Ok {
		return c.RenderJSON(re)
	}
	re.Ok, re.Msg = userService.UpdatePwd(c.getUserId(), oldPwd, pwd)
	return c.RenderJSON(re)
}

// 获得同步状态
// [OK]
func (c ApiUser) GetSyncState() revel.Result {
	ret := bson.M{"LastSyncUsn": userService.GetUsn(c.getUserId()), "LastSyncTime": time.Now().Unix()}
	return c.RenderJSON(ret)
}

// 头像设置
// 参数file=文件
// 成功返回{Logo: url} 头像新url
// [OK]
func (c ApiUser) UpdateLogo() revel.Result {
	isDemo, err := c.isDemoUser()
	if err != nil {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: apiDemoPolicyErrorMessage(err)})
	}
	if isDemo {
		return c.RenderJSON(info.ApiRe{Ok: false, Msg: "cannotUpdateDemo"})
	}
	ok, msg, url := c.uploadImage()

	if ok {
		if !userService.UpdateAvatar(c.getUserId(), url) {
			re := info.NewApiRe()
			re.Msg = "storage"
			return c.RenderJSON(re)
		}
		return c.RenderJSON(map[string]string{"Logo": url})
	} else {
		re := info.NewApiRe()
		re.Msg = msg
		return c.RenderJSON(re)
	}
}

// 上传图片
func (c ApiUser) uploadImage() (ok bool, msg, url string) {
	var fileUrlPath = ""
	ok = false

	var data []byte
	c.Params.Bind(&data, "file")
	files := c.Params.Files["file"]
	if len(files) == 0 {
		msg = "fileRequired"
		return
	}
	handel := files[0]
	if data == nil || len(data) == 0 {
		msg = "fileRequired"
		return
	}

	// file, handel, err := c.Request.FormFile("file")
	// if err != nil {
	// 	return
	// }
	// defer file.Close()
	// 生成上传路径
	fileUrlPath = "public/upload/" + c.getUserId() + "/images/logo"

	dir := revel.BasePath + "/" + fileUrlPath
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}
	// 生成新的文件名
	filename := handel.Filename

	var ext string

	_, ext = SplitFilename(filename)
	if ext != ".gif" && ext != ".jpg" && ext != ".png" && ext != ".bmp" && ext != ".jpeg" {
		msg = "notImage"
		return
	}

	filename = NewGuid() + ext
	// data, err := ioutil.ReadAll(file)
	// if err != nil {
	// 	LogJ(err)
	// 	return
	// }

	// > 5M?
	if len(data) > 5*1024*1024 {
		msg = "fileIsTooLarge"
		return
	}

	toPath := dir + "/" + filename
	err = ioutil.WriteFile(toPath, data, 0777)
	if err != nil {
		LogJ(err)
		return
	}

	ok = true
	url = configService.GetSiteUrl() + "/" + fileUrlPath + "/" + filename
	return
}

func (c ApiUser) isDemoUser() (bool, error) {
	return configService.IsDemoUser(c.getUserId())
}

func apiDemoPolicyErrorMessage(err error) string {
	if errors.Is(err, service.ErrDemoConfiguration) {
		return "configuration"
	}
	return "storage"
}
