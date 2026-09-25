package member

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/revel/revel"
	"github.com/revel/revel/session"
	"github.com/yangphere/leanote/app/controllers"
)

func TestUploadThemeImageRejectsMissingMultipartFile(t *testing.T) {
	memberBlog := newMemberBlogTestController(t, "/member/blog/uploadThemeImage")
	result := memberBlog.uploadImage("507f1f77bcf86cd799439012")
	if result.Ok {
		t.Fatalf("uploadImage without multipart file = %+v, want failure", result)
	}
}

func TestImportThemeRejectsMissingMultipartFile(t *testing.T) {
	memberBlog := newMemberBlogTestController(t, "/member/blog/importTheme")
	if result := memberBlog.ImportTheme(); result == nil {
		t.Fatal("ImportTheme returned a nil result")
	}
}

func newMemberBlogTestController(t *testing.T, path string) MemberBlog {
	t.Helper()
	ctx := revel.NewGoContext(nil)
	ctx.Request.SetRequest(httptest.NewRequest(http.MethodPost, path, nil))
	recorder := httptest.NewRecorder()
	ctx.Response.Original = recorder
	ctx.Response.SetWriter(recorder)
	controller := revel.NewController(ctx)
	controller.Session = session.NewSession()
	return MemberBlog{
		MemberBaseController: MemberBaseController{
			BaseController: controllers.BaseController{Controller: controller},
		},
	}
}
