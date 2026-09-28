# HTTP action inventory (B0 census, current registry snapshot)

Generated from `conf/routes` and exported `revel.Result` methods on 2026-09-25. The route and method census remains the migration baseline; the registry count below is maintained with the reconciliation test. `Golden` is intentionally `unrun` until the corresponding batch has replay evidence.

## Census

- routes: 95 (actions 83, static 9, catch-all 3)
- exported controller methods returning revel.Result: 242 (includes non-route helpers; see Routable column)
- current first-party registry: 249 actions when the production configuration is supplied (the complete routable action marker set, including main web/API, notes/content, publishing, member, and admin adapters); the reconciliation test reports `missing=0, extra=0`. A nil configuration omits only the production-only PDF action and is used only by focused unit fixtures.

The BEFORE/commonUrl column records the current Revel source facts, not the
future stdlib policy. The main controller interceptor is registered only for
`Notebook`, `Note`, `Share`, `User`, `Album`, `File`, `Attach`, and
`NoteContentHistory`; `Blog` is commented out and `Index` has no registration.
The API interceptor covers `ApiAuth`, `ApiUser`, `ApiFile`, `ApiNote`,
`ApiTag`, and `ApiNotebook`; its unauthenticated `commonUrl` entries are the
three `ApiAuth` actions and three `ApiFile` reads. Member and admin
interceptors are registered for their controller groups, while their
`needValidate`/commonUrl branches are currently bypassed by the interceptor
implementations. These facts are recorded so B1+ can explicitly preserve or
change each boundary.

## Route inventory

| # | Method | Path | Target | Kind | Owner | BEFORE/commonUrl | Response | Golden |
|---:|---|---|---|---|---|---|---|---|
| 1 | GET | `/_test/e2e/identity` | `TestE2e.Identity` | Action | interface-http | none (test endpoint applies run-mode/loopback checks) | Re/JSON/template (legacy) | unrun |
| 2 | GET | `/` | `Index.Default` | Action | application-identity | none (Index interceptor is not registered in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 3 | GET | `/note` | `Note.Index` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 4 | GET | `/index` | `Index.Index` | Action | application-identity | none (Index interceptor is not registered in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 5 | GET | `/login` | `Auth.Login` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 6 | POST | `/doLogin` | `Auth.DoLogin` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 7 | GET | `/logout` | `Auth.Logout` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 8 | GET | `/demo` | `Auth.Demo` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 9 | GET | `/register` | `Auth.Register` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 10 | POST | `/doRegister` | `Auth.DoRegister` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 11 | GET | `/findPassword/:token` | `Auth.FindPassword2` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 12 | GET | `/findPassword` | `Auth.FindPassword` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 13 | POST | `/doFindPassword` | `Auth.DoFindPassword` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 14 | POST | `/findPasswordUpdate` | `Auth.FindPasswordUpdate` | Action | application-identity | none (no Auth interceptor registration in controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 15 | * | `/note/listNotes` | `Note.ListNotes` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 16 | * | `/note/listTrashNotes` | `Note.ListTrashNotes` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 17 | * | `/note/getNoteAndContent` | `Note.GetNoteAndContent` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 18 | * | `/note/getNoteContent` | `Note.GetNoteContent` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 19 | * | `/note/updateNoteOrContent` | `Note.UpdateNoteOrContent` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 20 | * | `/note/deleteNote` | `Note.DeleteNote` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 21 | * | `/note/deleteTrash` | `Note.DeleteTrash` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 22 | * | `/note/moveNote` | `Note.MoveNote` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 23 | * | `/note/copyNote` | `Note.CopyNote` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 24 | * | `/note/copySharedNote` | `Note.CopySharedNote` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 25 | * | `/note/searchNoteByTags` | `Note.SearchNoteByTags` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 26 | * | `/note/setNote2Blog` | `Note.SetNote2Blog` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 27 | * | `/note/exportPdf` | `Note.ExportPDF` | Action | application-content | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 28 | * | `/note/toPdf` | `Note.ToPdf` | Action | application-content | controllers.AuthInterceptor; commonUrl exception Note.ToPdf | Re/JSON/template (legacy) | unrun |
| 29 | * | `/note/getNoteAndContentBySrc` | `Note.GetNoteAndContentBySrc` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/JSON/template (legacy) | unrun |
| 30 | GET | `/note/:noteId` | `Note.Index` | Action | application-notes | controllers.AuthInterceptor (controllers/init.go); commonUrl: none | HTML/Re/JSON (legacy) | unrun |
| 31 | GET | `/blog/getLikesAndComments` | `Blog.GetLikesAndComments` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 32 | GET | `/blog/getLikes` | `Blog.GetLikes` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 33 | * | `/blog/incReadNum` | `Blog.IncReadNum` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 34 | * | `/blog/likePost` | `Blog.LikePost` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 35 | * | `/blog/likeComment` | `Blog.LikeComment` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 36 | * | `/blog/deleteComment` | `Blog.DeleteComment` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 37 | GET | `/blog/getComments` | `Blog.GetComments` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 38 | * | `/blog/commentPost` | `Blog.CommentPost` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 39 | GET | `/blog/getPostStat` | `Blog.GetPostStat` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 40 | GET | `/blog/tags/:userIdOrEmail` | `Blog.Tags` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 41 | GET | `/blog/tags` | `Blog.Tags` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 42 | GET | `/blog/tag/:userIdOrEmail/:tag` | `Blog.Tag` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 43 | GET | `/blog/tag/:tag` | `Blog.Tag` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 44 | GET | `/blog/search/:userIdOrEmail` | `Blog.Search` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 45 | GET | `/blog/search` | `Blog.Search` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 46 | GET | `/blog/archives/:userIdOrEmail` | `Blog.Archives` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 47 | GET | `/blog/archives` | `Blog.Archives` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 48 | GET | `/blog/post/:noteId` | `Blog.Post` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 49 | GET | `/blog/post/:userIdOrEmail/:noteId` | `Blog.Post` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 50 | GET | `/blog/view/:noteId` | `Blog.Post` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 51 | GET | `/blog/single/:userIdOrEmail/:singleId` | `Blog.Single` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 52 | GET | `/blog/single/:singleId` | `Blog.Single` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 53 | GET | `/blog/cate/:notebookId` | `Blog.Cate` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 54 | GET | `/blog/cate/:userIdOrEmail/:notebookId` | `Blog.Cate` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 55 | GET | `/blog/listCateLatest/:notebookId` | `Blog.ListCateLatest` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 56 | GET | `/blog/:userIdOrEmail` | `Blog.Index` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 57 | GET | `/blog` | `Blog.Index` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 58 | GET | `/blog/*` | `Blog.E` | Action | application-publishing | none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | HTML/Re/JSON (legacy) | unrun |
| 59 | GET | `/preview/tags/:userIdOrEmail` | `Preview.Tags` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 60 | GET | `/preview/tags` | `Preview.Tags` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 61 | GET | `/preview/tag/:userIdOrEmail/:tag` | `Preview.Tag` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 62 | GET | `/preview/tag/:tag` | `Preview.Tag` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 63 | GET | `/preview/search/:userIdOrEmail` | `Preview.Search` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 64 | GET | `/preview/search` | `Preview.Search` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 65 | GET | `/preview/archives/:userIdOrEmail` | `Preview.Archives` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 66 | GET | `/preview/archives` | `Preview.Archives` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 67 | GET | `/preview/view/:noteId` | `Preview.Post` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 68 | GET | `/preview/post/:noteId` | `Preview.Post` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 69 | GET | `/preview/post/:userIdOrEmail/:noteId` | `Preview.Post` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 70 | GET | `/preview/single/:userIdOrEmail/:singleId` | `Preview.Single` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 71 | GET | `/preview/single/:singleId` | `Preview.Single` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 72 | GET | `/preview/cate/:notebookId` | `Preview.Cate` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 73 | GET | `/preview/cate/:userIdOrEmail/:notebookId` | `Preview.Cate` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 74 | GET | `/preview/:userIdOrEmail` | `Preview.Index` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 75 | GET | `/preview` | `Preview.Index` | Action | application-publishing | none (no controller interceptor registration) | HTML/Re/JSON (legacy) | unrun |
| 76 | GET | `/favicon.ico` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 77 | GET | `/public/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 78 | GET | `/js/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 79 | GET | `/images/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 80 | GET | `/img/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 81 | GET | `/css/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 82 | GET | `/fonts/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 83 | GET | `/tinymce/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 84 | GET | `/upload/*filepath` | `Static.Serve` | Static | interface-http | none (static handler; no controller BEFORE) | file/static | unrun |
| 85 | * | `/member` | `MemberIndex.Index` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 86 | * | `/member/index` | `MemberIndex.Index` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 87 | * | `/member/group/index` | `MemberGroup.Index` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 88 | * | `/member/group/addGroup` | `MemberGroup.AddGroup` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 89 | * | `/member/group/updateGroupTitle` | `MemberGroup.UpdateGroupTitle` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 90 | * | `/member/group/deleteGroup` | `MemberGroup.DeleteGroup` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 91 | * | `/member/group/addUser` | `MemberGroup.AddUser` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 92 | * | `/member/group/deleteUser` | `MemberGroup.DeleteUser` | Action | application-admin | member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/JSON/template (legacy) | unrun |
| 93 | * | `/:controller/:action` | `:controller.:action` | Catch-all | interface-http | prefix rewrite; interceptor depends on resolved controller | Re/JSON/template (legacy) | unrun |
| 94 | * | `/api/:controller/:action` | `:controller.:action` | Catch-all | interface-http | prefix rewrite; interceptor depends on resolved controller | Re/JSON/template (legacy) | unrun |
| 95 | * | `/member/:controller/:action` | `:controller.:action` | Catch-all | interface-http | prefix rewrite; interceptor depends on resolved controller | Re/JSON/template (legacy) | unrun |

## Exported controller method inventory

| Action | Routable basis | Owner | Method/BEFORE | Response | Golden |
|---|---|---|---|---|---|
| `Admin.GetView` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `Admin.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `Admin.T` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminBlog.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminBlog.SetRecommend` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.Backup` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.Delete` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.Download` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.Restore` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminData.UpdateRemark` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.Blog` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.DeleteEmails` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.Demo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.DoBlogTag` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.DoDemo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.DoToImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.Email` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.List` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.SendEmailDialog` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.SendEmailToEmails` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.SendToUsers` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.SendToUsers2` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.Set` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.Template` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminEmail.ToImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.Blog` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.Demo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.DoBlogTag` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.DoDemo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.DoSiteUrl` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.DoSubDomain` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.Email` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.ExportPdf` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.FeedbackRecipients` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.HomePage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.Mongodb` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.MongoExecutableAllowlist` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.OpenRegister` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.PdfExecutableAllowlist` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.ShareNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.SubDomain` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminSetting.UploadSize` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUpgrade.UpgradeBeta3ToBeta4` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUpgrade.UpgradeBetaToBeta2` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUpgrade.UpgradeBlog` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUser.Add` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUser.DoResetPwd` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUser.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUser.Register` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `AdminUser.ResetPwd` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / admin.AuthInterceptor (admin/init.go); commonUrl defined but current principal interceptor bypasses needValidate | Re/template/JSON | unrun |
| `Album.AddAlbum` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Album.DeleteAlbum` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Album.GetAlbums` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Album.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Album.UpdateAlbum` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `ApiAuth.Login` | catch-all candidate; stdlib registry `GET/POST` | application-identity | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiAuth.Logout` | catch-all candidate; stdlib registry `GET/POST` | application-identity | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiAuth.Register` | catch-all candidate; stdlib registry `POST` | application-identity | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiFile.GetAllAttachs` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / api.AuthInterceptor (api/init.go); commonUrl: ApiAuth.Login/Logout/Register, ApiFile.GetImage/GetAttach/GetAllAttachs | JSON | unrun |
| `ApiFile.GetAttach` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / api.AuthInterceptor (api/init.go); commonUrl: ApiAuth.Login/Logout/Register, ApiFile.GetImage/GetAttach/GetAllAttachs | JSON | unrun |
| `ApiFile.GetImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / api.AuthInterceptor (api/init.go); commonUrl: ApiAuth.Login/Logout/Register, ApiFile.GetImage/GetAttach/GetAllAttachs | JSON | unrun |
| `ApiNote.AddNote` | catch-all candidate; stdlib registry registered; multipart native replay passed 2026-09-28 | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | focused + Mongo/native replay passed; broader contract matrix remains in harness |
| `ApiNote.DeleteTrash` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.ExportPdf` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetHistories` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetNoteAndContent` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetNoteContent` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetNotes` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetSyncNotes` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.GetTrashNotes` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNote.UpdateNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNotebook.AddNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNotebook.DeleteNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNotebook.GetNotebooks` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNotebook.GetSyncNotebooks` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiNotebook.UpdateNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | unrun |
| `ApiTag.AddTag` | catch-all candidate; stdlib registry | application-notes | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiTag.DeleteTag` | catch-all candidate; stdlib registry | application-notes | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiTag.GetSyncTags` | catch-all candidate; stdlib registry | application-notes | * / api.AuthInterceptor ported to apiAuthBefore | JSON | contract passed; live replay blocked E-2 |
| `ApiUser.GetSyncState` | stdlib registry `POST` | application-identity | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | contract passed; live replay blocked E-2 |
| `ApiUser.Info` | stdlib registry `GET` | application-identity | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | contract passed; live replay blocked E-2 |
| `ApiUser.UpdateLogo` | stdlib registry `POST` | application-identity | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | multipart contract passed; live replay blocked E-2 |
| `ApiUser.UpdatePwd` | stdlib registry `POST` | application-identity | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | contract passed; live replay blocked E-2 |
| `ApiUser.UpdateUsername` | stdlib registry `POST` | application-identity | * / api.AuthInterceptor (api/init.go); commonUrl: none | JSON | contract passed; live replay blocked E-2 |
| `Attach.DeleteAttach` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `Attach.Download` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor; commonUrl exception Attach.Download | Re/file | unrun |
| `Attach.DownloadAll` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `Attach.GetAttachs` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `Attach.UploadAttach` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `Auth.Demo` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.DoFindPassword` | explicit route | application-identity | POST / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.DoLogin` | explicit route | application-identity | POST / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.DoRegister` | explicit route | application-identity | POST / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.FindPassword` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.FindPassword2` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.FindPasswordUpdate` | explicit route | application-identity | POST / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.Login` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.Logout` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Auth.Register` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `BaseController.E404` | helper only | interface-http | * / verify controller interceptor in source | Re/template/JSON | unrun |
| `BaseController.RenderRe` | helper only | interface-http | * / verify controller interceptor in source | Re/template/JSON | unrun |
| `Blog.CommentPost` | explicit route | application-publishing | * / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.DeleteComment` | explicit route | application-publishing | * / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.E` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.GetComments` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.GetLikes` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.GetLikesAndComments` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.GetPostStat` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.IncReadNum` | explicit route | application-publishing | * / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.LikeComment` | explicit route | application-publishing | * / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.LikePost` | explicit route | application-publishing | * / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Blog.ListCateLatest` | explicit route | application-publishing | GET / none (Blog interceptor is commented out in controllers/init.go); commonUrl map is legacy metadata only | Re/template/JSON | unrun |
| `Captcha.Get` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore | Re/image/JSON | contract passed; live unrun |
| `File.CopyHttpImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.CopyImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.DeleteImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.GetImages` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.OutputImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor; commonUrl exception File.OutputImage | Re/file | unrun |
| `File.PasteImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.UpdateImageTitle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.UploadAvatar` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.UploadBlogLogo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `File.UploadImageLeaui` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/file | unrun |
| `Index.Default` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Index.Index` | explicit route | application-identity | GET / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `Index.Suggestion` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore | Re/JSON | contract passed; live unrun |
| `MemberBlog.ActiveTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.AddOrUpdateSingle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Base` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Cate` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Comment` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DeleteSingle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DeleteTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DeleteThemeImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DeleteTpl` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DoAddOrUpdateSingle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.DoUpdateBlogAbstract` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.ExportTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.GetTplContent` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.ImportTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.InstallTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.ListThemeImages` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.NewTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Paging` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.PublicTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.SetUserBlogBase` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.SetUserBlogComment` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.SetUserBlogPaging` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.SetUserBlogStyle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Single` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.SortSingles` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.Theme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpateCateIds` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateBlogAbstract` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateBlogUrlTitle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateCateUrlTitle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateSingleUrlTitle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateTheme` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UpdateTplContent` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberBlog.UploadThemeImage` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.AddGroup` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.AddUser` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.DeleteGroup` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.DeleteUser` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.Index` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberGroup.UpdateGroupTitle` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberIndex.GetView` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberIndex.Index` | explicit route | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberIndex.T` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberUser.Avatar` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberUser.Email` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberUser.Password` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `MemberUser.Username` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-admin | * / member.AuthInterceptor (member/init.go); commonUrl defined but current session interceptor bypasses needValidate | Re/template/JSON | unrun |
| `Note.CopyNote` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.CopySharedNote` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.DeleteNote` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.DeleteTrash` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.ExportPdf` | explicit route | application-content | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.GetNoteAndContent` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.GetNoteAndContentBySrc` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.GetNoteContent` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.Index` | explicit route | application-notes | GET / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.ListNotes` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.ListTrashNotes` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.MoveNote` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.SearchNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.SearchNoteByTags` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.SetNote2Blog` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Note.ToPdf` | explicit route | application-content | * / controllers.AuthInterceptor; commonUrl exception Note.ToPdf | Re/template/JSON | unrun |
| `Note.UpdateNoteOrContent` | explicit route | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.AddNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.DeleteNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.DragNotebooks` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.GetNotebooks` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.Index` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.SetNotebook2Blog` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.SortNotebooks` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Notebook.UpdateNotebookTitle` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `NoteContentHistory.ListHistories` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Preview.Archives` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Cate` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Index` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Post` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Search` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Single` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Tag` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Preview.Tags` | explicit route | application-publishing | GET / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Share.AddShareNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.AddShareNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.AddShareNotebookGroup` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.AddShareNoteGroup` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNote` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNotebookBySharedUser` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNotebookGroup` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNoteBySharedUser` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteShareNoteGroup` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.DeleteUserShareNoteAndNotebook` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.GetShareNoteContent` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.ListNotebookShareUserInfo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.ListNoteShareUserInfo` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.ListShareNotes` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.UpdateShareNotebookGroupPerm` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.UpdateShareNotebookPerm` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.UpdateShareNoteGroupPerm` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Share.UpdateShareNotePerm` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-publishing | * / controllers.AuthInterceptor (controllers/init.go); commonUrl: none | Re/template/JSON | unrun |
| `Tag.DeleteTag` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `Tag.UpdateTag` | catch-all candidate; stdlib registry registered; Golden/live replay unrun | application-notes | * / none (no controller interceptor registration) | Re/template/JSON | unrun |
| `TestE2e.Identity` | explicit route | interface-http | GET / none (test endpoint applies run-mode/loopback checks) | Re/template/JSON | unrun |
| `User.Account` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.ActiveEmail` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `User.ReSendActiveEmail` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.SendRegisterEmail` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.UpdateColumnWidth` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.UpdateEmail` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore | Re/template/JSON | contract passed; live unrun |
| `User.UpdateLeftIsMin` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.UpdatePwd` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.UpdateTheme` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |
| `User.UpdateUsername` | catch-all candidate; first-party registered in B2 | application-identity | * / first-party webSessionBefore + requireWebAuthentication | Re/template/JSON | contract passed; live unrun |

## Registry reconciliation input (test fixture)

The following list is the full routable action set used by `app/controllers/httpserver_inventory_test.go`: the 240 exported controller methods classified as catch-all candidates or explicit routes, plus nine route aliases for embedded Blog methods and the `Note.ExportPDF` route spelling. It excludes the two `BaseController` helper-only methods, Static routes, and `:controller.:action` placeholders. The test is expected to fail in B0 while migration batches have not registered these actions.

<!-- ROUTABLE_ACTIONS_BEGIN -->
TestE2e.Identity
Index.Default
Note.Index
Index.Index
Auth.Login
Auth.DoLogin
Auth.Logout
Auth.Demo
Auth.Register
Auth.DoRegister
Auth.FindPassword2
Auth.FindPassword
Auth.DoFindPassword
Auth.FindPasswordUpdate
Note.ListNotes
Note.ListTrashNotes
Note.GetNoteAndContent
Note.GetNoteContent
Note.UpdateNoteOrContent
Note.DeleteNote
Note.DeleteTrash
Note.MoveNote
Note.CopyNote
Note.CopySharedNote
Note.SearchNoteByTags
Note.SetNote2Blog
Note.ExportPDF
Note.ToPdf
Note.GetNoteAndContentBySrc
Blog.GetLikesAndComments
Blog.GetLikes
Blog.IncReadNum
Blog.LikePost
Blog.LikeComment
Blog.DeleteComment
Blog.GetComments
Blog.CommentPost
Blog.GetPostStat
Blog.Tags
Blog.Tag
Blog.Search
Blog.Archives
Blog.Post
Blog.Single
Blog.Cate
Blog.ListCateLatest
Blog.Index
Blog.E
Preview.Tags
Preview.Tag
Preview.Search
Preview.Archives
Preview.Post
Preview.Single
Preview.Cate
Preview.Index
MemberIndex.Index
MemberGroup.Index
MemberGroup.AddGroup
MemberGroup.UpdateGroupTitle
MemberGroup.DeleteGroup
MemberGroup.AddUser
MemberGroup.DeleteUser
ApiAuth.Login
ApiAuth.Logout
ApiAuth.Register
ApiTag.GetSyncTags
ApiTag.AddTag
ApiTag.DeleteTag
Admin.GetView
Admin.Index
Admin.T
AdminBlog.Index
AdminBlog.SetRecommend
AdminData.Backup
AdminData.Delete
AdminData.Download
AdminData.Index
AdminData.Restore
AdminData.UpdateRemark
AdminEmail.Blog
AdminEmail.DeleteEmails
AdminEmail.Demo
AdminEmail.DoBlogTag
AdminEmail.DoDemo
AdminEmail.DoToImage
AdminEmail.Email
AdminEmail.List
AdminEmail.SendEmailDialog
AdminEmail.SendEmailToEmails
AdminEmail.SendToUsers
AdminEmail.SendToUsers2
AdminEmail.Set
AdminEmail.Template
AdminEmail.ToImage
AdminSetting.Blog
AdminSetting.Demo
AdminSetting.DoBlogTag
AdminSetting.DoDemo
AdminSetting.DoSiteUrl
AdminSetting.DoSubDomain
AdminSetting.Email
AdminSetting.ExportPdf
AdminSetting.FeedbackRecipients
AdminSetting.HomePage
AdminSetting.Mongodb
AdminSetting.MongoExecutableAllowlist
AdminSetting.OpenRegister
AdminSetting.PdfExecutableAllowlist
AdminSetting.ShareNote
AdminSetting.SubDomain
AdminSetting.UploadSize
AdminUpgrade.UpgradeBeta3ToBeta4
AdminUpgrade.UpgradeBetaToBeta2
AdminUpgrade.UpgradeBlog
AdminUser.Add
AdminUser.DoResetPwd
AdminUser.Index
AdminUser.Register
AdminUser.ResetPwd
Album.AddAlbum
Album.DeleteAlbum
Album.GetAlbums
Album.Index
Album.UpdateAlbum
ApiFile.GetAllAttachs
ApiFile.GetAttach
ApiFile.GetImage
ApiNote.AddNote
ApiNote.DeleteTrash
ApiNote.ExportPdf
ApiNote.GetHistories
ApiNote.GetNote
ApiNote.GetNoteAndContent
ApiNote.GetNoteContent
ApiNote.GetNotes
ApiNote.GetSyncNotes
ApiNote.GetTrashNotes
ApiNote.UpdateNote
ApiNotebook.AddNotebook
ApiNotebook.DeleteNotebook
ApiNotebook.GetNotebooks
ApiNotebook.GetSyncNotebooks
ApiNotebook.UpdateNotebook
ApiUser.GetSyncState
ApiUser.Info
ApiUser.UpdateLogo
ApiUser.UpdatePwd
ApiUser.UpdateUsername
Attach.DeleteAttach
Attach.Download
Attach.DownloadAll
Attach.GetAttachs
Attach.UploadAttach
Captcha.Get
File.CopyHttpImage
File.CopyImage
File.DeleteImage
File.GetImages
File.OutputImage
File.PasteImage
File.UpdateImageTitle
File.UploadAvatar
File.UploadBlogLogo
File.UploadImageLeaui
Index.Suggestion
MemberBlog.ActiveTheme
MemberBlog.AddOrUpdateSingle
MemberBlog.Base
MemberBlog.Cate
MemberBlog.Comment
MemberBlog.DeleteSingle
MemberBlog.DeleteTheme
MemberBlog.DeleteThemeImage
MemberBlog.DeleteTpl
MemberBlog.DoAddOrUpdateSingle
MemberBlog.DoUpdateBlogAbstract
MemberBlog.ExportTheme
MemberBlog.GetTplContent
MemberBlog.ImportTheme
MemberBlog.Index
MemberBlog.InstallTheme
MemberBlog.ListThemeImages
MemberBlog.NewTheme
MemberBlog.Paging
MemberBlog.PublicTheme
MemberBlog.SetUserBlogBase
MemberBlog.SetUserBlogComment
MemberBlog.SetUserBlogPaging
MemberBlog.SetUserBlogStyle
MemberBlog.Single
MemberBlog.SortSingles
MemberBlog.Theme
MemberBlog.UpateCateIds
MemberBlog.UpdateBlogAbstract
MemberBlog.UpdateBlogUrlTitle
MemberBlog.UpdateCateUrlTitle
MemberBlog.UpdateSingleUrlTitle
MemberBlog.UpdateTheme
MemberBlog.UpdateTplContent
MemberBlog.UploadThemeImage
MemberIndex.GetView
MemberIndex.T
MemberUser.Avatar
MemberUser.Email
MemberUser.Password
MemberUser.Username
Note.ExportPdf
Note.SearchNote
Notebook.AddNotebook
Notebook.DeleteNotebook
Notebook.DragNotebooks
Notebook.GetNotebooks
Notebook.Index
Notebook.SetNotebook2Blog
Notebook.SortNotebooks
Notebook.UpdateNotebookTitle
NoteContentHistory.ListHistories
Share.AddShareNote
Share.AddShareNotebook
Share.AddShareNotebookGroup
Share.AddShareNoteGroup
Share.DeleteShareNote
Share.DeleteShareNotebook
Share.DeleteShareNotebookBySharedUser
Share.DeleteShareNotebookGroup
Share.DeleteShareNoteBySharedUser
Share.DeleteShareNoteGroup
Share.DeleteUserShareNoteAndNotebook
Share.GetShareNoteContent
Share.ListNotebookShareUserInfo
Share.ListNoteShareUserInfo
Share.ListShareNotes
Share.UpdateShareNotebookGroupPerm
Share.UpdateShareNotebookPerm
Share.UpdateShareNoteGroupPerm
Share.UpdateShareNotePerm
Tag.DeleteTag
Tag.UpdateTag
User.Account
User.ActiveEmail
User.ReSendActiveEmail
User.SendRegisterEmail
User.UpdateColumnWidth
User.UpdateEmail
User.UpdateLeftIsMin
User.UpdatePwd
User.UpdateTheme
User.UpdateUsername
<!-- ROUTABLE_ACTIONS_END -->
