package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/revel"
	"github.com/revel/revel/session"
)

func TestFileUploadImageRejectsMissingMultipartFile(t *testing.T) {
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/file/uploadAvatar", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()

	result := (File{BaseController: BaseController{Controller: controller}}).uploadImage("logo", "")
	if result.Ok {
		t.Fatalf("uploadImage without multipart file = %+v, want failure", result)
	}
}
