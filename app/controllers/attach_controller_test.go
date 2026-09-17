package controllers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/revel"
	"github.com/revel/revel/session"
)

func TestAttachUploadRejectsMalformedNoteIDWithoutPanic(t *testing.T) {
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, "/attach/uploadAttach", nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["UserId"] = "507f1f77bcf86cd799439011"

	result := (Attach{BaseController: BaseController{Controller: controller}}).uploadAttach("not-an-object-id")
	if result.Ok || result.Id != "" {
		t.Fatalf("uploadAttach with malformed note id = %+v, want failure", result)
	}
}
