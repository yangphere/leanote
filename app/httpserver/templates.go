package httpserver

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	i18n "github.com/yangphere/leanote/app/lea/i18n"
)

// currentLocaleViewArg is the ViewArgs key carrying the request locale
// (revel.CurrentLocaleViewArg value, kept for template compatibility).
const currentLocaleViewArg = "currentLocale"

// TemplateFuncs returns the 27 active template functions in the exact
// shape app/init.go registered with Revel. The name set is frozen by
// TestTemplateFuncsNameSet.
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"raw": func(str string) template.HTML {
			return template.HTML(str)
		},
		"trim": func(str string) string {
			str = strings.Trim(str, " ")
			str = strings.Trim(str, " ")

			str = strings.Trim(str, "\n")
			str = strings.Trim(str, "&nbsp;")

			// 以下两个空格不一样
			str = strings.Trim(str, " ")
			str = strings.Trim(str, " ")
			return str
		},
		"add": func(i int) string {
			i = i + 1
			return fmt.Sprintf("%v", i)
		},
		"sub": func(i int) int {
			i = i - 1
			return i
		},
		// 增加或减少
		"incr": func(n, i int) int {
			n = n + i
			return n
		},
		"join": func(arr []string) template.HTML {
			if arr == nil {
				return template.HTML("")
			}
			return template.HTML(strings.Join(arr, ","))
		},
		"concat": func(s1, s2 string) template.HTML {
			return template.HTML(s1 + s2)
		},
		"concatStr": func(strs ...string) string {
			str := ""
			for _, s := range strs {
				str += s
			}
			return str
		},
		"decodeUrlValue": func(i string) string {
			v, _ := url.ParseQuery("a=" + i)
			return v.Get("a")
		},
		"json": func(i interface{}) string {
			b, _ := json.Marshal(i)
			return string(b)
		},
		"jsonJs": func(i interface{}) template.JS {
			b, _ := json.Marshal(i)
			return template.JS(string(b))
		},
		"datetime": func(t time.Time) template.HTML {
			return template.HTML(t.Format("2006-01-02 15:04:05"))
		},
		"dateFormat": func(t time.Time, format string) template.HTML {
			return template.HTML(t.Format(format))
		},
		"unixDatetime": func(unixSec string) template.HTML {
			sec, _ := strconv.Atoi(unixSec)
			t := time.Unix(int64(sec), 0)
			return template.HTML(t.Format("2006-01-02 15:04:05"))
		},
		// Revel builtins the views rely on (template_functions.go subset).
		"set": func(viewArgs map[string]interface{}, key string, value interface{}) template.JS {
			if viewArgs != nil {
				viewArgs[key] = value
			}
			return template.JS("")
		},
		"append": func(viewArgs map[string]interface{}, key string, value interface{}) template.JS {
			if viewArgs == nil {
				return template.JS("")
			}
			if viewArgs[key] == nil {
				viewArgs[key] = []interface{}{value}
			} else {
				viewArgs[key] = append(viewArgs[key].([]interface{}), value)
			}
			return template.JS("")
		},
		"pad": func(str string, width int) template.HTML {
			if len(str) >= width {
				return template.HTML(html.EscapeString(str))
			}
			return template.HTML(html.EscapeString(str) + strings.Repeat("&nbsp;", width-len(str)))
		},
		// interface是否有该字段
		"has": func(i interface{}, key string) bool {
			t := reflect.TypeOf(i)
			_, ok := t.FieldByName(key)
			return ok
		},
		"blogTags": func(renderArgs map[string]interface{}, tags []string) template.HTML {
			if tags == nil || len(tags) == 0 {
				return ""
			}
			locale, _ := renderArgs[currentLocaleViewArg].(string)
			tagStr := ""
			lenTags := len(tags)

			tagPostUrl, _ := renderArgs["tagPostsUrl"].(string)

			for i, tag := range tags {
				str := i18n.Message(locale, tag)
				var classes = "label"
				if strings.HasPrefix(str, "???") {
					str = tag
				}
				if inArray([]string{"red", "blue", "yellow", "green"}, tag) {
					classes += " label-" + tag
				} else {
					classes += " label-default"
				}

				classes += " label-post"
				var url = tagPostUrl + "/" + tag
				tagStr += "<a class=\"" + classes + "\" href=\"" + url + "\">" + str + "</a>"
				if i != lenTags-1 {
					tagStr += " "
				}
			}
			return template.HTML(tagStr)
		},
		"blogTagsForExport": func(renderArgs map[string]interface{}, tags []string) template.HTML {
			if tags == nil || len(tags) == 0 {
				return ""
			}
			tagStr := ""
			lenTags := len(tags)

			for i, tag := range tags {
				str := tag
				var classes = "label"
				if inArray([]string{"red", "blue", "yellow", "green"}, tag) {
					classes += " label-" + tag
				} else {
					classes += " label-default"
				}

				classes += " label-post"
				tagStr += "<span class=\"" + classes + "\" >" + str + "</span>"
				if i != lenTags-1 {
					tagStr += " "
				}
			}
			return template.HTML(tagStr)
		},
		"msg": func(renderArgs map[string]interface{}, message string, args ...interface{}) template.HTML {
			str, ok := renderArgs[currentLocaleViewArg].(string)
			if !ok {
				return ""
			}
			return template.HTML(i18n.Message(str, message, args...))
		},
		// 不用revel的msg
		"leaMsg": func(renderArgs map[string]interface{}, key string) template.HTML {
			locale, _ := renderArgs[currentLocaleViewArg].(string)
			str := i18n.Message(locale, key)
			if strings.HasPrefix(str, "???") {
				str = key
			}
			return template.HTML(str)
		},
		// lea++
		"blogTagsLea": func(renderArgs map[string]interface{}, tags []string, typeStr string) template.HTML {
			if tags == nil || len(tags) == 0 {
				return ""
			}
			locale, _ := renderArgs[currentLocaleViewArg].(string)
			tagStr := ""
			lenTags := len(tags)

			tagPostUrl := "http://lea.leanote.com/"
			if typeStr == "recommend" {
				tagPostUrl += "?tag="
			} else if typeStr == "latest" {
				tagPostUrl += "latest?tag="
			} else {
				tagPostUrl += "subscription?tag="
			}

			for i, tag := range tags {
				str := i18n.Message(locale, tag)
				var classes = "label"
				if strings.HasPrefix(str, "???") {
					str = tag
				}
				if inArray([]string{"red", "blue", "yellow", "green"}, tag) {
					classes += " label-" + tag
				} else {
					classes += " label-default"
				}
				classes += " label-post"
				var url = tagPostUrl + tag
				tagStr += "<a class=\"" + classes + "\" href=\"" + url + "\">" + str + "</a>"
				if i != lenTags-1 {
					tagStr += " "
				}
			}
			return template.HTML(tagStr)
		},
		"li": func(a string) string {
			return ""
		},
		// str连接
		"urlConcat": func(url string, v ...interface{}) string {
			html := ""
			for i := 0; i < len(v); i = i + 2 {
				item := v[i]
				if i+1 == len(v) {
					break
				}
				value := v[i+1]
				if item != nil && value != nil {
					keyStr, _ := item.(string)
					valueStr, err := value.(string)
					if !err {
						valueInt, _ := value.(int)
						valueStr = strconv.Itoa(valueInt)
					}
					if keyStr != "" && valueStr != "" {
						s := keyStr + "=" + valueStr
						if html != "" {
							html += "&" + s
						} else {
							html += s
						}
					}
				}
			}

			if html != "" {
				if strings.Index(url, "?") >= 0 {
					return url + "&" + html
				}
				return url + "?" + html
			}
			return url
		},
		"urlCond": func(url string, sorterI, keyords interface{}) template.HTML {
			return ""
		},
		// 返回HTMLAttr, 返回html, golang 会执行安全检查返回ZgotmplZ
		"rawMsg": func(renderArgs map[string]interface{}, message string, args ...interface{}) template.JS {
			str, ok := renderArgs[currentLocaleViewArg].(string)
			if !ok {
				return ""
			}
			return template.JS(i18n.Message(str, message, args...))
		},
		// 为后台管理sorter th使用（必须 HTMLAttr，否则 Go 会输出 ZgotmplZ）
		"sorterTh": func(url, sorterField string, sorterI interface{}) template.HTMLAttr {
			sorter := ""
			if sorterI != nil {
				sorter, _ = sorterI.(string)
			}
			html := "data-url=\"" + url + "\" data-sorter=\"" + sorterField + "\""
			html += " class=\"th-sortable "
			if sorter == sorterField+"-up" {
				html += "th-sort-up\""
			} else if sorter == sorterField+"-down" {
				html += "th-sort-down"
			}
			html += "\""
			return template.HTMLAttr(html)
		},
		// pagination
		"page": func(urlBase string, page, pageSize, count int) template.HTML {
			if count == 0 {
				return ""
			}
			totalPage := int(math.Ceil(float64(count) / float64(pageSize)))

			preClass := ""
			prePage := page - 1
			if prePage == 0 {
				prePage = 1
			}
			nextClass := ""
			nextPage := page + 1
			var preUrl, nextUrl string

			preUrl = urlBase + "?page=" + strconv.Itoa(prePage)
			nextUrl = urlBase + "?page=" + strconv.Itoa(nextPage)

			// 没有上一页了
			if page == 1 {
				preClass = "disabled"
				preUrl = "#"
			}
			// 没有下一页了
			if totalPage <= page {
				nextClass = "disabled"
				nextUrl = "#"
			}
			return template.HTML("<li class='" + preClass + "'><a href='" + preUrl + "'>Previous</a></li> <li  class='" + nextClass + "'><a href='" + nextUrl + "'>Next</a></li>")
		},
		"N": func(start, end int) (stream chan int) {
			stream = make(chan int)
			go func() {
				for i := start; i <= end; i++ {
					stream <- i
				}
				close(stream)
			}()
			return
		},
	}
}

// inArray is lea.InArray kept local so templates.go depends only on
// httpserver + lea/i18n.
func inArray(arr []string, str string) bool {
	for _, v := range arr {
		if v == str {
			return true
		}
	}
	return false
}

// LoadTemplates walks dir (an app views tree, e.g. app/views) and parses
// every .html file into one template set keyed by slash-relative path
// ("errors/404.html"), with the project template funcs installed.
func LoadTemplates(dir string) (*template.Template, error) {
	tpl := template.New("").Funcs(TemplateFuncs())
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		name, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		name = filepath.ToSlash(name)
		_, perr := tpl.New(name).ParseFiles(path)
		return perr
	})
	if err != nil {
		return nil, err
	}
	return tpl, nil
}

// TemplateSetRenderer adapts a parsed template set to the
// httpserver.TemplateRenderer hook (render by name with ViewArgs).
func TemplateSetRenderer(tpl *template.Template) func(name string, args map[string]interface{}) ([]byte, error) {
	return func(name string, args map[string]interface{}) ([]byte, error) {
		t := tpl.Lookup(name)
		if t == nil {
			return nil, fmt.Errorf("template %q not found", name)
		}
		var b strings.Builder
		if err := t.Execute(&b, args); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	}
}
