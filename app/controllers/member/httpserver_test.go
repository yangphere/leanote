package member

import (
	"net/http/httptest"
	"testing"

	"github.com/yangphere/leanote/app/httpserver"
)

func TestRegisterHTTPIncludesMemberActions(t *testing.T) {
	r := httpserver.NewRegistry()
	RegisterHTTP(r)
	for _, action := range []string{
		"MemberIndex.Index", "MemberIndex.GetView", "MemberGroup.AddGroup",
		"MemberGroup.DeleteUser", "MemberUser.Avatar", "MemberBlog.Index",
		"MemberBlog.UpdateTplContent", "MemberBlog.SetUserBlogPaging",
	} {
		controller, method := splitAction(action)
		entry, ok := r.Lookup(controller, method)
		if !ok || entry.Handler == nil {
			t.Fatalf("missing member action %s", action)
		}
	}
}

func TestMemberAuthUsesSharedPrincipal(t *testing.T) {
	c := &httpserver.Context{Request: httptest.NewRequest("GET", "/member", nil)}
	c.SetPrincipal(httpserver.AuthenticatedPrincipal("507f1f77bcf86cd799439011", httpserver.PrincipalSourceWebSession, httpserver.TokenStateAbsent))
	if result := memberAuthBefore(c); result != nil {
		t.Fatal("authenticated shared principal was rejected by member auth boundary")
	}
}

func splitAction(action string) (string, string) {
	for i := 0; i < len(action); i++ {
		if action[i] == '.' {
			return action[:i], action[i+1:]
		}
	}
	return action, ""
}
