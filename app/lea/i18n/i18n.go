package i18n

import (
	"fmt"
	"github.com/revel/revel"
	"github.com/robfig/config"
	. "github.com/yangphere/leanote/app/lea"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	CurrentLocaleViewArg = "currentLocale" // The key for the current locale render arg value

	messageFilesDirectory = "messages"
	messageFilePattern    = `^\w+\.conf$`
	unknownValueFormat    = "??? %s ???"
	defaultLanguageOption = "i18n.default_language"
	localeCookieConfigKey = "i18n.cookie"
)

var (
	// All currently loaded message configs.
	// en-us, zh-cn, zh-hk ->
	messages map[string]*config.Config
)

func GetAllLangMessages() map[string]*config.Config {
	return messages
}

func HasLang(lang string) bool {
	_, ok := messages[lang]
	return ok
}

func GetDefaultLang() string {
	lang, _ := revel.Config.String(defaultLanguageOption)
	return lang
}

// Return all currently loaded message languages.
func MessageLanguages() []string {
	languages := make([]string, len(messages))
	i := 0
	for language, _ := range messages {
		languages[i] = language
		i++
	}
	return languages
}

// Perform a message look-up for the given locale and message using the given arguments.
//
// When either an unknown locale or message is detected, a specially formatted string is returned.
// DefaultLanguage is the fallback language when the requested locale has
// no message; settable by plain-Go processes (Revel reads
// i18n.default_language from app.conf).
var DefaultLanguage = ""

func Message(locale, message string, args ...interface{}) string {
	language, region := parseLocale(locale)

	langAndRegion := language + "-" + region
	// revel.TRACE.Println(langAndRegion + " 怎么回事")

	messageConfig, knownLanguage := messages[langAndRegion]
	if !knownLanguage {
		// Default language resolution: i18n.default_language under Revel;
		// the settable DefaultLanguage var in plain-Go processes (seam).
		defaultLanguage := DefaultLanguage
		if defaultLanguage == "" && revel.Config != nil {
			if v, found := revel.Config.String(defaultLanguageOption); found && v != "" {
				defaultLanguage = v
			}
		}
		if defaultLanguage == "" {
			return fmt.Sprintf(unknownValueFormat, message)
		}

		messageConfig, knownLanguage = messages[defaultLanguage]
		if !knownLanguage {
			return fmt.Sprintf(unknownValueFormat, message)
		}
	}

	// This works because unlike the goconfig documentation suggests it will actually
	// try to resolve message in DEFAULT if it did not find it in the given section.
	value, error := messageConfig.String(region, message)
	if error != nil {
		// WARN.Printf("Unknown message '%s' for locale '%s'", message, locale)
		return fmt.Sprintf(unknownValueFormat, message)
	}

	if len(args) > 0 {
		// revel.TRACE.Printf("Arguments detected, formatting '%s' with %v", value, args)
		value = fmt.Sprintf(value, args...)
	}

	return value
}

func parseLocale(locale string) (language, region string) {
	if strings.Contains(locale, "-") {
		languageAndRegion := strings.Split(locale, "-")
		return languageAndRegion[0], languageAndRegion[1]
	}

	return locale, ""
}

// Recursively read and cache all available messages from all message files on the given path.
func loadMessages(path string) {
	messages = make(map[string]*config.Config)

	if error := filepath.Walk(path, loadEachMessageLang); error != nil && !os.IsNotExist(error) {
		// ERROR.Println("Error reading messages files:", error)
	}
}

// 加载每一个文件夹
func loadEachMessageLang(parentPath string, parentInfo os.FileInfo, osError error) (err error) {
	if !parentInfo.IsDir() {
		return nil
	}

	if err := filepath.Walk(parentPath, func(path string, info os.FileInfo, osError error) error {
		return loadMessageFile(parentInfo.Name(), path, info, osError)

	}); err != nil && !os.IsNotExist(err) {
		// ERROR.Println("Error reading messages files:", error)
	}
	return err
}

// Load a single message file
func loadMessageFile(locale string, path string, info os.FileInfo, osError error) error {
	if osError != nil {
		return osError
	}
	if info.IsDir() {
		return nil
	}

	if matched, _ := regexp.MatchString(messageFilePattern, info.Name()); matched {
		if config, error := parseMessagesFile(path); error != nil {
			return error
		} else {
			// locale := parseLocaleFromFileName(info.Name())
			// revel.TRACE.Print(locale + "----locale")

			// If we have already parsed a message file for this locale, merge both
			if _, exists := messages[locale]; exists {
				messages[locale].Merge(config)
				Logf("Successfully merged messages for locale '%s'", locale)
			} else {
				messages[locale] = config
			}

			Logf("Successfully loaded messages from file: %s", info.Name())
		}
	} else {
		Logf("Ignoring file %s because it did not have a valid extension", info.Name())
	}

	return nil
}

func parseMessagesFile(path string) (messageConfig *config.Config, error error) {
	messageConfig, error = config.ReadDefault(path)
	return
}

func parseLocaleFromFileName(file string) string {
	extension := filepath.Ext(file)[1:]
	return strings.ToLower(extension)
}

func init() {
	revel.OnAppStart(func() {
		loadMessages(filepath.Join(revel.BasePath, messageFilesDirectory))
	})
}

// LoadMessages loads message files from dir for plain-Go processes
// (revel used OnAppStart with BasePath + the messages dir name).
func LoadMessages(dir string) {
	loadMessages(dir)
}

func I18nFilter(c *revel.Controller, fc []revel.Filter) {
	if foundCookie, cookieValue := hasLocaleCookie(c.Request); foundCookie {
		// revel.TRACE.Printf("Found locale cookie value: %s", cookieValue)
		setCurrentLocaleControllerArguments(c, cookieValue)
	} else if foundHeader, headerValue := hasAcceptLanguageHeader(c.Request); foundHeader {
		// revel.TRACE.Printf("Found Accept-Language header value: %s", headerValue)
		setCurrentLocaleControllerArguments(c, headerValue)
	} else {
		// revel.TRACE.Println("Unable to find locale in cookie or header, using empty string")
		setCurrentLocaleControllerArguments(c, "")
	}
	fc[0](c, fc[1:])
}

// Set the current locale controller argument (CurrentLocaleControllerArg) with the given locale.
func setCurrentLocaleControllerArguments(c *revel.Controller, locale string) {
	c.Request.Locale = locale
	c.ViewArgs[CurrentLocaleViewArg] = locale
}

// Determine whether the given request has valid Accept-Language value.
//
// Assumes that the accept languages stored in the request are sorted according to quality, with top
// quality first in the slice.
func hasAcceptLanguageHeader(request *revel.Request) (bool, string) {
	if request.AcceptLanguages != nil && len(request.AcceptLanguages) > 0 {
		return true, request.AcceptLanguages[0].Language
	}

	return false, ""
}

// Determine whether the given request has a valid language cookie value.
func hasLocaleCookie(request *revel.Request) (bool, string) {
	if request != nil {
		name := revel.Config.StringDefault(localeCookieConfigKey, revel.CookiePrefix+"_LANG")
		if cookie, error := request.Cookie(name); error == nil {
			return true, cookie.GetValue()
		} else {
			// revel.TRACE.Printf("Unable to read locale cookie with name '%s': %s", name, error.Error())
		}
	}

	return false, ""
}
