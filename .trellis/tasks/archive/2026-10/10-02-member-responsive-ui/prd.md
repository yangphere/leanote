# 优化管理后台与个人中心响应式布局

## Goal

重整 Leanote 管理后台（管理员 `/admin` 与个人中心 `/member`）的共享外壳、页面间距和主要表单/卡片的响应式布局与视觉层次，确保常见桌面与窄屏宽度下内容不溢出、可读且保持现有交互。

## Confirmed Facts

- The shared member shell is assembled in `app/views/member/top.html`: a fixed top header, a `220px` `aside-md` navigation column, and a scrollable content section (`top.html:16-18`, `top.html:48`, `top.html:87`).
- The shared admin shell has the same fixed header/sidebar/content pattern in `app/views/admin/top.html` (`top.html:16-18`, `:51`, `:88-90`) and serves its own `public/admin/css/admin.css`.
- The shell hides the whole user link list below the Bootstrap `sm` breakpoint (`top.html:24`), while the sidebar remains a desktop-oriented `aside-md` layout.
- The admin shell hides both its aside and nav with `d-none d-sm-block` (`app/views/admin/top.html:25`, `:51`), so narrow widths need an explicit navigation presentation.
- The group page uses Bootstrap grid columns (`col-sm-4`) in both the jsrender `#tGroup` template and server-rendered `{{range .groups}}` branch (`app/views/member/group/index.html:28`, `:55`) and defines a later inline `width: 200px` rule for `.group-title` (`:2-10`), so narrow content widths can force clipping or cramped cards.
- Runtime styles are loaded from `public/admin/css/admin.css` and `public/member/css/member.css`; the editable sources are the matching `.less` files.
- Existing group actions are delegated through the `group` jQuery module in the same group template and must continue to work.

## Requirements

- Keep existing routes, group CRUD behavior, member navigation links, and localized labels unchanged.
- Make both shared admin/member shells usable from narrow mobile widths through wide desktop widths: header content must wrap or collapse without horizontal overflow, navigation must remain reachable, and content must have predictable spacing beside or below navigation.
- Make the group page readable at narrow widths: group cards should use the available width, titles should remain editable without forcing overflow, long member identities should wrap safely, and cards should flow into sensible columns as space allows.
- Improve visual hierarchy with consistent page spacing, card radius/border/shadow, control sizing, focus states, and navigation active states while staying compatible with the existing Bootstrap 5 styles.
- Limit changes to the `/admin` and `/member` templates/styles needed for this visual and responsive fix; do not alter backend contracts or the note editor/public blog UI.

## Acceptance Criteria

- [ ] At widths around 375px, 600px, 768px, 1024px, and 1440px, both `/admin` and `/member` have no horizontal page overflow caused by their headers, sidebars, content wrappers, tables, forms, or cards.
- [ ] At narrow widths, both shells keep the brand and account links usable, the navigation does not cover the content, and page titles/actions remain reachable.
- [ ] Group cards use a single-column layout when space is narrow and expand to multiple columns on larger widths; editable titles, member rows, delete actions, and add-member inputs remain inside each card. The `#tGroup` jsrender branch and server-rendered branch use the same grid class, and the inline fixed-width `.group-title` rule is removed or rewritten.
- [ ] Existing group interactions (create group, rename on blur, delete group, add member on Enter, delete member) still bind to the same selectors and endpoints.
- [ ] The appended responsive rule block in each `.less`/`.css` pair is pure CSS and byte-for-byte identical, covering both `admin.less`/`admin.css` and `member.less`/`member.css`; the changed files pass formatting/diff checks plus the most targeted available frontend tests.
- [ ] Browser overflow validation checks `scrollWidth <= clientWidth` on `#content`, `.scrollable`, and `.card` elements (excluding intentional `.table-responsive` scrolling), rather than relying on the presence of a page scrollbar.

## Out of Scope

- Rewriting the member-center information architecture or adding new account features.
- Changing backend group APIs, permissions, translations, or unrelated note editor/public blog pages.

## Key Decisions

- “整个管理后台” means `/admin` and `/member`, including their shared shells and management tables/forms/cards.
- The note editor and public blog surfaces remain out of scope.
- Responsive behavior is implemented in the two existing LESS/CSS bundles and small template class adjustments; backend routes and page interactions remain unchanged.

## Planning Status

- `design.md` and `implement.md` describe the shared shell strategy, group-card grid, risk points, and validation sequence.
- `implement.jsonl` and `check.jsonl` reference the frontend quality guidelines required by the Trellis sub-agents.
- No blocking product or compatibility questions remain.
