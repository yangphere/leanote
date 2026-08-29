package i18n

import (
	"bufio"
	"fmt"
	"github.com/revel/revel"
	"github.com/robfig/config"
	. "github.com/yangphere/leanote/app/lea"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
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
func loadMessages(path string) error {
	messages = make(map[string]*config.Config)

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("messages directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("messages path %q is not a directory", path)
	}
	if err := filepath.Walk(path, loadEachMessageLang); err != nil {
		return fmt.Errorf("load messages from %q: %w", path, err)
	}
	return nil
}

// 加载每一个文件夹
func loadEachMessageLang(parentPath string, parentInfo os.FileInfo, osError error) (err error) {
	if osError != nil {
		return osError
	}
	if parentInfo == nil {
		return fmt.Errorf("message path %q has no file info", parentPath)
	}
	if !parentInfo.IsDir() {
		return nil
	}

	if err := filepath.Walk(parentPath, func(path string, info os.FileInfo, osError error) error {
		return loadMessageFile(parentInfo.Name(), path, info, osError)

	}); err != nil {
		return err
	}
	return nil
}

// Load a single message file
func loadMessageFile(locale string, path string, info os.FileInfo, osError error) error {
	if osError != nil {
		return osError
	}
	if info == nil {
		return fmt.Errorf("message file %q has no file info", path)
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
	if err := validateMessageSyntax(path); err != nil {
		return nil, err
	}
	messageConfig, error = config.ReadDefault(path)
	return
}

// validateMessageSyntax catches the parser errors from robfig/config before
// it opens the file. The frozen dependency does not close its file handle on
// parse errors, which is observable as an undeletable file on Windows.
func validateMessageSyntax(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	section := ""
	option := ""
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimRightFunc(stripMessageComments(scanner.Text()), unicode.IsSpace)
		if len(line) == 0 || line[0] == '#' || line[0] == ';' {
			continue
		}
		if line[0] == '[' && line[len(line)-1] == ']' {
			section = strings.TrimSpace(line[1 : len(line)-1])
			option = ""
			continue
		}
		if section != "" && option != "" && (line[0] == ' ' || line[0] == '\t') {
			continue
		}
		separator := strings.IndexAny(line, "=:")
		if separator <= 0 || line[0] == ' ' || line[0] == '\t' {
			return fmt.Errorf("could not parse line %d in %q: %s", lineNumber, path, line)
		}
		option = strings.TrimSpace(line[:separator])
	}
	return scanner.Err()
}

func stripMessageComments(line string) string {
	for _, marker := range []string{" ;", "\t;", " #", "\t#"} {
		if index := strings.Index(line, marker); index != -1 {
			line = line[:index]
		}
	}
	return line
}

func parseLocaleFromFileName(file string) string {
	extension := filepath.Ext(file)[1:]
	return strings.ToLower(extension)
}

func init() {
	revel.OnAppStart(func() {
		if err := loadMessages(filepath.Join(revel.BasePath, messageFilesDirectory)); err != nil {
			panic(err)
		}
	})
}

// LoadMessages loads message files from dir for plain-Go processes
// (revel used OnAppStart with BasePath + the messages dir name).
func LoadMessages(dir string) error {
	return loadMessages(dir)
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
