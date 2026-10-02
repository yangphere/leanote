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

func TestMemberIndexRendersTemplatesWithoutErrors(t *testing.T) {
	templates, err := httpserver.LoadTemplates("../../views")
	if err != nil {
		t.Fatal(err)
	}
	previousRenderer := httpserver.TemplateRenderer
	t.Cleanup(func() { httpserver.TemplateRenderer = previousRenderer })
	httpserver.TemplateRenderer = httpserver.TemplateSetRenderer(templates)

	r := httpserver.NewRegistry()
	RegisterHTTP(r)
	entry, ok := r.Lookup("MemberIndex", "Index")
	if !ok || entry.Handler == nil {
		t.Fatal("missing MemberIndex.Index handler")
	}

	for _, tc := range []struct {
		name      string
		principal httpserver.Principal
	}{
		{
			name:      "regular user",
			principal: httpserver.AuthenticatedPrincipal("507f1f77bcf86cd799439011", httpserver.PrincipalSourceWebSession, httpserver.TokenStateAbsent),
		},
		{
			name: "admin user",
			principal: httpserver.Principal{
				UserID: "507f1f77bcf86cd799439011",
				Role:   httpserver.PrincipalRoleAdmin,
				Source: httpserver.PrincipalSourceWebSession,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/member", nil)
			w := httptest.NewRecorder()
			c := &httpserver.Context{Request: req}
			c.SetPrincipal(tc.principal)

			res := entry.Handler(c)
			if res == nil {
				t.Fatal("expected handler result")
			}
			res.Apply(w, req)
			if w.Code != 200 {
				t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
			}
		})
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

