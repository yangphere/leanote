package controllers

import (
	"net/http/httptest"
	"testing"

	"github.com/revel/revel"
	"github.com/revel/revel/session"
)

func TestPreviewInvalidThemeIDDoesNotReplaceSession(t *testing.T) {
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest("GET", "/preview/index/not-an-id", nil))
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["UserId"] = "507f1f77bcf86cd799439011"
	controller.Session["themeId"] = "507f1f77bcf86cd799439012"

	preview := Preview{Blog: Blog{BaseController: BaseController{Controller: controller}}}
	if preview.getPreviewThemeAbsolutePath("not-an-id") {
		t.Fatal("preview accepted an invalid theme ID")
	}
	if got := preview.GetSession("themeId"); got != "507f1f77bcf86cd799439012" {
		t.Fatalf("preview replaced the previous theme ID with %q", got)
	}
}

func TestPreviewOwnerPathMustMatchAuthenticatedUser(t *testing.T) {
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest("GET", "/preview/507f1f77bcf86cd799439011", nil))
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	controller.Session["UserId"] = "507f1f77bcf86cd799439011"
	preview := Preview{Blog: Blog{BaseController: BaseController{Controller: controller}}}
	if !preview.matchesPreviewOwner("507f1f77bcf86cd799439011") {
		t.Fatal("preview rejected the authenticated owner")
	}
	if preview.matchesPreviewOwner("507f1f77bcf86cd799439012") {
		t.Fatal("preview accepted a foreign owner in the URL")
	}
}
