package lea

import "testing"

// Frozen outputs of SubStringHTML captured from goquery v1.6.1 behaviour on
// 2026-08-26 (Phase 1 of 08-25-go-toolchain). The upgrade to goquery v1.12.0
// must keep every case byte-identical.
func TestSubStringHTMLFrozenContract(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		length int
		end    string
		want   string
	}{
		{"empty", "", 10, "", ""},
		{"short-passthrough", "hello", 10, "", "hello"},
		{"plain-truncate", "hello world", 5, "", "hell"},
		{"multibyte-count", "<p>你好世界，测试</p>", 3, "", "<p>你</p>"},
		{"nested-tags-completed",
			`<div class="x"><p>abcdef</p><p>ghijkl</p></div>`, 8, "",
			`<div class="x"><p>abcde</p></div>`},
		{"incomplete-tag-dropped", "abc<img src='a.png' />def", 4, "", "abc"},
		{"entities", "&nbsp;&amp;hello", 4, "", "\u00a0&amp;h"},
		{"void-img", `<p>a<img src="x"/>b</p>`, 3, "", "<p>a</p>"},
		{"bare-gt-escaped", "a>b c", 5, "", "a&gt;b c"},
		{"list-with-end", "<ul><li>one</li><li>two</li></ul>", 6, "...", "<ul><li>one</li></ul>"},
		{"script-head-wrapper", "<script>var a=1;</script>body", 5, "",
			"<html><head><script>var</script></head><body>"},
		{"unterminated-div", "<div><span>hi</span>", 3, "", "<div><span></span></div>"},
		{"chinese-end-suffix", "<p>第一段</p><p>第二段</p>", 4, "……", "<p>第一……</p>"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := SubStringHTML(c.input, c.length, c.end)
			if got != c.want {
				t.Errorf("SubStringHTML(%q,%d,%q) = %q, want %q", c.input, c.length, c.end, got, c.want)
			}
		})
	}
}

// SubStringHTMLToRaw is the plain-text sibling used beside the HTML digest.
func TestSubStringHTMLToRawBasics(t *testing.T) {
	if got := SubStringHTMLToRaw("", 10); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := SubStringHTMLToRaw("<p>ab<b>cd</b></p>", 3); got != "ab" {
		t.Errorf("raw truncate = %q, want ab (tag chars consume length budget)", got)
	}
	if got := SubStringHTMLToRaw("你好世界", 2); got != "你好" {
		t.Errorf("rune slice = %q, want 你好", got)
	}
}
