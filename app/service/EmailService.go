package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	. "github.com/yangphere/leanote/app/lea"
	"go.mongodb.org/mongo-driver/v2/bson"
	"html"
	"html/template"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 发送邮件

type EmailService struct {
	tplMu sync.RWMutex
	tpls  map[string]*template.Template
	send  func(context.Context, string, string, string) error
}

func tokenTimeoutHours(tokenType int) string {
	return strconv.Itoa(int(tokenTTL(tokenType).Hours()))
}

func NewEmailService() *EmailService {
	return &EmailService{tpls: map[string]*template.Template{}}
}

// 发送邮件
var host = ""
var emailPort = ""
var username = ""
var password = ""
var ssl = false

func InitEmailFromDb() {
	host = configService.GetGlobalStringConfig("emailHost")
	emailPort = configService.GetGlobalStringConfig("emailPort")
	username = configService.GetGlobalStringConfig("emailUsername")
	password = configService.GetGlobalStringConfig("emailPassword")
	if configService.GetGlobalStringConfig("emailSSL") == "1" {
		ssl = true
	}
}

// return a smtp client
func dial(addr string) (*smtp.Client, error) {
	conn, err := tls.Dial("tcp", addr, nil)
	if err != nil {
		LogW("Dialing Error:", err)
		return nil, err
	}
	//分解主机端口字符串
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}

func SendEmailWithSSL(auth smtp.Auth, to []string, msg []byte) (err error) {
	//create smtp client
	c, err := dial(host + ":" + emailPort)
	if err != nil {
		LogW("Create smpt client error:", err)
		return err
	}
	defer c.Close()

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(auth); err != nil {
				LogW("Error during AUTH", err)
				return err
			}
		}
	}

	if err = c.Mail(username); err != nil {
		return err
	}

	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	_, err = w.Write(msg)
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return c.Quit()
}

func (this *EmailService) SendEmail(to, subject, body string) (ok bool, e string) {
	InitEmailFromDb()

	if host == "" || emailPort == "" || username == "" || password == "" {
		return
	}
	hp := strings.Split(host, ":")
	auth := smtp.PlainAuth("", username, password, hp[0])

	var content_type string

	mailtype := "html"
	if mailtype == "html" {
		content_type = "Content-Type: text/" + mailtype + "; charset=UTF-8"
	} else {
		content_type = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	msg := []byte("To: " + to + "\r\nFrom: " + username + "<" + username + ">\r\nSubject: " + subject + "\r\n" + content_type + "\r\n\r\n" + body)
	send_to := strings.Split(to, ";")

	var err error
	if ssl {
		err = SendEmailWithSSL(auth, send_to, msg)
	} else {
		Log("no ssl")
		err = smtp.SendMail(host+":"+emailPort, auth, username, send_to, msg)
	}

	if err != nil {
		e = fmt.Sprint(err)
		return
	}
	ok = true
	return
}

type smtpDeliveryConfig struct {
	host     string
	port     string
	username string
	password string
	ssl      bool
}

func currentSMTPDeliveryConfig() (smtpDeliveryConfig, error) {
	if configService == nil {
		return smtpDeliveryConfig{}, errors.New("email configuration service is not initialized")
	}
	config := smtpDeliveryConfig{
		host:     strings.TrimSpace(configService.GetGlobalStringConfig("emailHost")),
		port:     strings.TrimSpace(configService.GetGlobalStringConfig("emailPort")),
		username: strings.TrimSpace(configService.GetGlobalStringConfig("emailUsername")),
		password: configService.GetGlobalStringConfig("emailPassword"),
		ssl:      configService.GetGlobalStringConfig("emailSSL") == "1",
	}
	if config.host == "" || config.port == "" || config.username == "" || config.password == "" {
		return smtpDeliveryConfig{}, errors.New("email transport configuration is incomplete")
	}
	return config, nil
}

// SendEmailContext is the cancellable SMTP boundary used by the outbox
// worker. It preserves the existing direct-SMTP behavior while bounding dial
// and protocol I/O by the worker context.
func (this *EmailService) SendEmailContext(ctx context.Context, to, subject, body string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return fmt.Errorf("%w: email header contains a line break", db.ErrOutboxTransportRejected)
	}
	config, err := currentSMTPDeliveryConfig()
	if err != nil {
		return fmt.Errorf("%w: %w", db.ErrOutboxTransportRejected, err)
	}
	recipients := strings.Split(to, ";")
	for _, recipient := range recipients {
		if !IsEmail(strings.TrimSpace(recipient)) {
			return fmt.Errorf("%w: email recipient is invalid", db.ErrOutboxTransportRejected)
		}
	}
	contentType := "Content-Type: text/html; charset=UTF-8"
	message := []byte("To: " + to + "\r\nFrom: " + config.username + "<" + config.username + ">\r\nSubject: " + subject + "\r\n" + contentType + "\r\n\r\n" + body)
	return sendSMTPContext(ctx, config, recipients, message)
}

func sendSMTPContext(ctx context.Context, config smtpDeliveryConfig, recipients []string, message []byte) error {
	address := net.JoinHostPort(config.host, config.port)
	dialer := &net.Dialer{}
	var conn net.Conn
	var err error
	if config.ssl {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: &tls.Config{ServerName: config.host}}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("%w: dial SMTP: %w", db.ErrOutboxTransportRejected, err)
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("%w: set SMTP deadline: %w", db.ErrOutboxTransportRejected, err)
		}
	}
	stopCancellation := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	defer stopCancellation()

	client, err := smtp.NewClient(conn, config.host)
	if err != nil {
		return fmt.Errorf("%w: create SMTP client: %w", db.ErrOutboxTransportRejected, err)
	}
	defer client.Close()
	if !config.ssl {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: config.host}); err != nil {
				return fmt.Errorf("%w: start SMTP TLS: %w", db.ErrOutboxTransportRejected, err)
			}
		}
	}
	auth := smtp.PlainAuth("", config.username, config.password, config.host)
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("%w: authenticate SMTP: %w", db.ErrOutboxTransportRejected, err)
		}
	}
	if err := client.Mail(config.username); err != nil {
		return fmt.Errorf("%w: set SMTP sender: %w", db.ErrOutboxTransportRejected, err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(strings.TrimSpace(recipient)); err != nil {
			return fmt.Errorf("%w: set SMTP recipient: %w", db.ErrOutboxTransportRejected, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("%w: open SMTP body: %w", db.ErrOutboxTransportRejected, err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP body: %w", err)
	}
	if err := writer.Close(); err != nil {
		var response *textproto.Error
		if errors.As(err, &response) {
			return fmt.Errorf("%w: close SMTP body: %w", db.ErrOutboxTransportRejected, err)
		}
		return fmt.Errorf("close SMTP body: %w", err)
	}
	_ = client.Quit()
	return nil
}

// DeliverOutbox renders and sends a supported email event using the token
// already committed in the event payload. It never issues or resolves a new
// token while delivering a side effect.
func (this *EmailService) DeliverOutbox(ctx context.Context, event db.OutboxEvent) error {
	if this == nil {
		if event.Kind == "comment" {
			return fmt.Errorf("%w: email service is not initialized", db.ErrOutboxTransportRejected)
		}
		return errors.New("email service is not initialized")
	}
	if configService == nil {
		if event.Kind == "comment" {
			return fmt.Errorf("%w: email configuration service is not initialized", db.ErrOutboxTransportRejected)
		}
		return errors.New("email configuration service is not initialized")
	}
	email := ""
	if event.Kind != "feedback" {
		var err error
		email, err = outboxPayloadString(event.Payload, "email")
		if err != nil || !IsEmail(email) {
			if event.Kind == "comment" {
				return fmt.Errorf("%w: outbox email payload is invalid", db.ErrOutboxTransportRejected)
			}
			return errors.New("outbox email payload is invalid")
		}
	}
	var subject, body string
	var values map[string]interface{}
	plainTextBody := false
	switch event.Kind {
	case "feedback":
		recipients, ok := outboxPayloadStrings(event.Payload, "recipients")
		if !ok || len(recipients) == 0 || len(recipients) > 20 {
			return fmt.Errorf("%w: feedback recipients are invalid", db.ErrOutboxTransportRejected)
		}
		for _, recipient := range recipients {
			if !IsEmail(recipient) || strings.ContainsAny(recipient, "\r\n") {
				return fmt.Errorf("%w: feedback recipient is invalid", db.ErrOutboxTransportRejected)
			}
		}
		subject, _ = event.Payload["subject"].(string)
		if strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") {
			return fmt.Errorf("%w: feedback subject is invalid", db.ErrOutboxTransportRejected)
		}
		feedbackBody, _ := event.Payload["body"].(string)
		if strings.TrimSpace(feedbackBody) == "" {
			return fmt.Errorf("%w: feedback body is invalid", db.ErrOutboxTransportRejected)
		}
		addr, _ := event.Payload["addr"].(string)
		body = "<p>Suggestion:</p><pre>" + html.EscapeString(feedbackBody) + "</pre>"
		if addr != "" {
			body += "<p>Contact: " + html.EscapeString(addr) + "</p>"
		}
		send := this.send
		if send == nil {
			send = this.SendEmailContext
		}
		if err := send(ctx, strings.Join(recipients, ";"), subject, body); err != nil {
			return fmt.Errorf("send feedback email: %w", err)
		}
		return nil
	case "broadcast":
		var ok bool
		subject, ok = event.Payload["subject"].(string)
		if !ok || strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") {
			return fmt.Errorf("%w: broadcast subject is invalid", db.ErrOutboxTransportRejected)
		}
		body, ok = event.Payload["body"].(string)
		if !ok || strings.TrimSpace(body) == "" {
			return fmt.Errorf("%w: broadcast body is invalid", db.ErrOutboxTransportRejected)
		}
		send := this.send
		if send == nil {
			send = this.SendEmailContext
		}
		if err := send(ctx, email, subject, body); err != nil {
			return fmt.Errorf("send broadcast email: %w", err)
		}
		return nil
	case "comment":
		comment, commentOK := event.Payload["content"].(string)
		if !commentOK || strings.TrimSpace(comment) == "" {
			return fmt.Errorf("%w: comment outbox content is invalid", db.ErrOutboxTransportRejected)
		}
		subject = "New blog comment"
		body = "A new comment was posted on your blog:\n\n" + html.EscapeString(comment)
		values = map[string]interface{}{}
		plainTextBody = true
	case "activate-email":
		token, err := outboxPayloadString(event.Payload, "token")
		if err != nil {
			return err
		}
		tokenType, err := outboxPayloadInt(event.Payload, "tokenType")
		if err != nil {
			return err
		}
		if tokenType != info.TokenActiveEmail {
			return errors.New("activation outbox token purpose is invalid")
		}
		userID, err := outboxPayloadString(event.Payload, "userId")
		if err != nil || userID != event.AggregateID.Hex() {
			return errors.New("activation outbox identity is invalid")
		}
		username, err := outboxPayloadString(event.Payload, "username")
		if err != nil {
			return err
		}
		subject = configService.GetGlobalStringConfig("emailTemplateRegisterSubject")
		body = configService.GetGlobalStringConfig("emailTemplateRegister")
		values = map[string]interface{}{
			"tokenUrl":     configService.GetSiteUrl() + "/user/activeEmail?token=" + token,
			"token":        token,
			"tokenTimeout": tokenTimeoutHours(info.TokenActiveEmail),
			"user": map[string]interface{}{
				"userId": userID, "email": email, "username": username,
			},
		}
	case "reset-password":
		token, err := outboxPayloadString(event.Payload, "token")
		if err != nil {
			return err
		}
		tokenType, err := outboxPayloadInt(event.Payload, "tokenType")
		if err != nil {
			return err
		}
		if tokenType != info.TokenPwd {
			return errors.New("password-reset outbox token purpose is invalid")
		}
		userID, err := outboxPayloadString(event.Payload, "userId")
		if err != nil || userID != event.AggregateID.Hex() {
			return errors.New("password-reset outbox identity is invalid")
		}
		subject = configService.GetGlobalStringConfig("emailTemplateFindPasswordSubject")
		body = configService.GetGlobalStringConfig("emailTemplateFindPassword")
		values = map[string]interface{}{
			"tokenUrl":     configService.GetSiteUrl() + "/findPassword/" + token,
			"token":        token,
			"tokenTimeout": tokenTimeoutHours(info.TokenPwd),
		}
	default:
		return errors.New("unsupported outbox event kind")
	}
	if strings.TrimSpace(body) == "" {
		return errors.New("outbox email template is empty")
	}
	if plainTextBody {
		send := this.send
		if send == nil {
			send = this.SendEmailContext
		}
		if err := send(ctx, email, subject, body); err != nil {
			return fmt.Errorf("send outbox email: %w", err)
		}
		return nil
	}
	ok, message, renderedSubject, renderedBody := this.renderEmail(subject, body, values)
	if !ok {
		return fmt.Errorf("render outbox email: %s", message)
	}
	send := this.send
	if send == nil {
		send = this.SendEmailContext
	}
	if err := send(ctx, email, renderedSubject, renderedBody); err != nil {
		return fmt.Errorf("send outbox email: %w", err)
	}
	return nil
}

func outboxPayloadString(payload map[string]any, key string) (string, error) {
	value, ok := payload[key].(string)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("outbox payload %s is invalid", key)
	}
	return value, nil
}

func outboxPayloadStrings(payload map[string]any, key string) ([]string, bool) {
	value, ok := payload[key]
	if !ok {
		return nil, false
	}
	result := make([]string, 0)
	switch values := value.(type) {
	case []string:
		result = append(result, values...)
	case []any:
		for _, item := range values {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			result = append(result, strings.TrimSpace(text))
		}
	default:
		return nil, false
	}
	return result, true
}

func outboxPayloadInt(payload map[string]any, key string) (int, error) {
	switch value := payload[key].(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	default:
		return 0, fmt.Errorf("outbox payload %s is invalid", key)
	}
}

// AddUser调用
// 可以使用一个goroutine
func (this *EmailService) RegisterSendActiveEmail(userInfo info.User, email string) bool {
	token := tokenService.NewToken(userInfo.UserId.Hex(), email, info.TokenActiveEmail)
	if token == "" {
		return false
	}

	subject := configService.GetGlobalStringConfig("emailTemplateRegisterSubject")
	tpl := configService.GetGlobalStringConfig("emailTemplateRegister")

	if tpl == "" {
		return false
	}

	tokenUrl := configService.GetSiteUrl() + "/user/activeEmail?token=" + token
	// {siteUrl} {tokenUrl} {token} {tokenTimeout} {user.id} {user.email} {user.username}
	token2Value := map[string]interface{}{"siteUrl": configService.GetSiteUrl(), "tokenUrl": tokenUrl, "token": token, "tokenTimeout": tokenTimeoutHours(info.TokenActiveEmail),
		"user": map[string]interface{}{
			"userId":   userInfo.UserId.Hex(),
			"email":    userInfo.Email,
			"username": userInfo.Username,
		},
	}

	var ok bool
	ok, _, subject, tpl = this.renderEmail(subject, tpl, token2Value)
	if !ok {
		return false
	}

	// 发送邮件
	ok, _ = this.SendEmail(email, subject, tpl)
	return ok
}

// 修改邮箱
func (this *EmailService) UpdateEmailSendActiveEmail(userInfo info.User, email string) (ok bool, msg string) {
	// 先验证该email是否被注册了
	if userService.IsExistsUser(email) {
		ok = false
		msg = "该邮箱已注册"
		return
	}

	token := tokenService.NewToken(userInfo.UserId.Hex(), email, info.TokenUpdateEmail)

	if token == "" {
		return
	}

	subject := configService.GetGlobalStringConfig("emailTemplateUpdateEmailSubject")
	tpl := configService.GetGlobalStringConfig("emailTemplateUpdateEmail")

	// 发送邮件
	tokenUrl := configService.GetSiteUrl() + "/user/updateEmail?token=" + token
	// {siteUrl} {tokenUrl} {token} {tokenTimeout} {user.userId} {user.email} {user.username}
	token2Value := map[string]interface{}{"siteUrl": configService.GetSiteUrl(), "tokenUrl": tokenUrl, "token": token, "tokenTimeout": tokenTimeoutHours(info.TokenUpdateEmail),
		"newEmail": email,
		"user": map[string]interface{}{
			"userId":   userInfo.UserId.Hex(),
			"email":    userInfo.Email,
			"username": userInfo.Username,
		},
	}

	ok, msg, subject, tpl = this.renderEmail(subject, tpl, token2Value)
	if !ok {
		return
	}

	// 发送邮件
	ok, msg = this.SendEmail(email, subject, tpl)
	return
}

func (this *EmailService) FindPwdSendEmail(token, email string) (ok bool, msg string) {
	subject := configService.GetGlobalStringConfig("emailTemplateFindPasswordSubject")
	tpl := configService.GetGlobalStringConfig("emailTemplateFindPassword")

	// 发送邮件
	tokenUrl := configService.GetSiteUrl() + "/findPassword/" + token
	// {siteUrl} {tokenUrl} {token} {tokenTimeout} {user.id} {user.email} {user.username}
	token2Value := map[string]interface{}{"siteUrl": configService.GetSiteUrl(), "tokenUrl": tokenUrl,
		"token": token, "tokenTimeout": tokenTimeoutHours(info.TokenPwd)}

	ok, msg, subject, tpl = this.renderEmail(subject, tpl, token2Value)
	if !ok {
		return
	}
	// 发送邮件
	ok, msg = this.SendEmail(email, subject, tpl)
	return
}

// 发送邀请链接
func (this *EmailService) SendInviteEmail(userInfo info.User, email, content string) bool {
	subject := configService.GetGlobalStringConfig("emailTemplateInviteSubject")
	tpl := configService.GetGlobalStringConfig("emailTemplateInvite")

	token2Value := map[string]interface{}{"siteUrl": configService.GetSiteUrl(),
		"registerUrl": configService.GetSiteUrl() + "/register?from=" + userInfo.Username,
		"content":     content,
		"user": map[string]interface{}{
			"username": userInfo.Username,
			"email":    userInfo.Email,
		},
	}
	var ok bool
	ok, _, subject, tpl = this.renderEmail(subject, tpl, token2Value)
	if !ok {
		return false
	}
	// 发送邮件
	ok, _ = this.SendEmail(email, subject, tpl)
	return ok
}

// 发送评论
func (this *EmailService) SendCommentEmail(note info.Note, comment info.BlogComment, userId, content string) bool {
	subject := configService.GetGlobalStringConfig("emailTemplateCommentSubject")
	tpl := configService.GetGlobalStringConfig("emailTemplateComment")

	// title := "评论提醒"

	/*
		toUserId := note.UserId.Hex()
		// title := "评论提醒"

		// 表示回复回复的内容, 那么发送给之前回复的
		if comment.CommentId != "" {
			toUserId = comment.UserId.Hex()
		}
		toUserInfo := userService.GetUserInfo(toUserId)
		sendUserInfo := userService.GetUserInfo(userId)

		subject := note.Title + " 收到 " + sendUserInfo.Username + " 的评论";
		if comment.CommentId != "" {
			subject = "您在 " + note.Title + " 发表的评论收到 " + sendUserInfo.Username;
			if userId == note.UserId.Hex() {
				subject += "(作者)";
			}
			subject += " 的评论";
		}
	*/

	toUserId := note.UserId.Hex()
	// 表示回复回复的内容, 那么发送给之前回复的
	if !comment.CommentId.IsZero() {
		toUserId = comment.UserId.Hex()
	}
	toUserInfo := userService.GetUserInfo(toUserId) // 被评论者
	sendUserInfo := userService.GetUserInfo(userId) // 评论者

	// {siteUrl} {blogUrl}
	// {blog.id} {blog.title} {blog.url}
	// {commentUser.userId} {commentUser.username} {commentUser.email}
	// {commentedUser.userId} {commentedUser.username} {commentedUser.email}
	token2Value := map[string]interface{}{"siteUrl": configService.GetSiteUrl(), "blogUrl": configService.GetBlogUrl(),
		"blog": map[string]string{
			"id":    note.NoteId.Hex(),
			"title": note.Title,
			"url":   configService.GetBlogUrl() + "/view/" + note.NoteId.Hex(),
		},
		"commentContent": content,
		// 评论者信息
		"commentUser": map[string]interface{}{"userId": sendUserInfo.UserId.Hex(),
			"username":     sendUserInfo.Username,
			"email":        sendUserInfo.Email,
			"isBlogAuthor": userId == note.UserId.Hex(),
		},
		// 被评论者信息
		"commentedUser": map[string]interface{}{"userId": toUserId,
			"username":     toUserInfo.Username,
			"email":        toUserInfo.Email,
			"isBlogAuthor": toUserId == note.UserId.Hex(),
		},
	}

	ok := false
	ok, _, subject, tpl = this.renderEmail(subject, tpl, token2Value)
	if !ok {
		return false
	}

	// 发送邮件
	ok, _ = this.SendEmail(toUserInfo.Email, subject, tpl)
	return ok
}

// 验证模板是否正确
func (this *EmailService) ValidTpl(str string) (ok bool, msg string) {
	defer func() {
		if err := recover(); err != nil {
			ok = false
			msg = fmt.Sprint(err)
		}
	}()
	header := configService.GetGlobalStringConfig("emailTemplateHeader")
	footer := configService.GetGlobalStringConfig("emailTemplateFooter")
	str = strings.Replace(str, "{{header}}", header, -1)
	str = strings.Replace(str, "{{footer}}", footer, -1)
	_, err := template.New("tpl name").Parse(str)
	if err != nil {
		msg = fmt.Sprint(err)
		return
	}
	ok = true
	return
}

// ok, msg, subject, tpl
func (this *EmailService) getTpl(str string) (ok bool, msg string, tpl *template.Template) {
	defer func() {
		if err := recover(); err != nil {
			ok = false
			msg = fmt.Sprint(err)
		}
	}()

	var err error
	var has bool

	this.tplMu.RLock()
	tpl, has = this.tpls[str]
	this.tplMu.RUnlock()
	if !has {
		tpl, err = template.New("tpl name").Parse(str)
		if err != nil {
			msg = fmt.Sprint(err)
			return
		}
		this.tplMu.Lock()
		if existing, exists := this.tpls[str]; exists {
			tpl = existing
		} else {
			this.tpls[str] = tpl
		}
		this.tplMu.Unlock()
	}
	ok = true
	return
}

// 通过subject, body和值得到内容
func (this *EmailService) renderEmail(subject, body string, values map[string]interface{}) (ok bool, msg string, o string, b string) {
	ok = false
	msg = ""
	defer func() { // 必须要先声明defer，否则不能捕获到panic异常
		if err := recover(); err != nil {
			ok = false
			msg = fmt.Sprint(err) // 这里的err其实就是panic传入的内容，
		}
	}()

	var tpl *template.Template

	values["siteUrl"] = configService.GetSiteUrl()

	// subject
	if subject != "" {
		ok, msg, tpl = this.getTpl(subject)
		if !ok {
			return
		}
		var buffer bytes.Buffer
		err := tpl.Execute(&buffer, values)
		if err != nil {
			msg = fmt.Sprint(err)
			return
		}
		o = buffer.String()
	} else {
		o = ""
	}

	// content
	header := configService.GetGlobalStringConfig("emailTemplateHeader")
	footer := configService.GetGlobalStringConfig("emailTemplateFooter")
	body = strings.Replace(body, "{{header}}", header, -1)
	body = strings.Replace(body, "{{footer}}", footer, -1)
	values["subject"] = o
	ok, msg, tpl = this.getTpl(body)
	if !ok {
		return
	}
	var buffer2 bytes.Buffer
	err := tpl.Execute(&buffer2, values)
	if err != nil {
		msg = fmt.Sprint(err)
		return
	}
	b = buffer2.String()

	return
}

// 发送email给用户
// 需要记录
func (this *EmailService) SendEmailToUsers(users []info.User, subject, body string) (ok bool, msg string) {
	if users == nil || len(users) == 0 {
		msg = "no users"
		return
	}

	// 尝试renderHtml
	ok, msg, _, _ = this.renderEmail(subject, body, map[string]interface{}{})
	if !ok {
		Log(msg)
		return
	}

	for _, user := range users {
		LogJ(user)
		m := map[string]interface{}{}
		m["userId"] = user.UserId.Hex()
		m["username"] = user.Username
		m["email"] = user.Email
		ok2, msg2, subject2, body2 := this.renderEmail(subject, body, m)
		ok = ok2
		msg = msg2
		if ok2 {
			sendOk, msg := this.SendEmail(user.Email, subject2, body2)
			this.AddEmailLog(user.Email, subject, body, sendOk, msg) // 把模板记录下
			// 记录到Email Log
			if sendOk {
				// Log("ok " + user.Email)
			} else {
				// Log("no " + user.Email)
			}
		} else {
			// Log(msg);
		}
	}

	return
}

func (this *EmailService) SendEmailToEmails(emails []string, subject, body string) (ok bool, msg string) {
	if emails == nil || len(emails) == 0 {
		msg = "no emails"
		return
	}

	// 尝试renderHtml
	ok, msg, _, _ = this.renderEmail(subject, body, map[string]interface{}{})
	if !ok {
		Log(msg)
		return
	}

	//	go func() {
	for _, email := range emails {
		if email == "" {
			continue
		}
		m := map[string]interface{}{}
		m["email"] = email
		ok, msg, subject, body = this.renderEmail(subject, body, m)
		if ok {
			sendOk, msg := this.SendEmail(email, subject, body)
			this.AddEmailLog(email, subject, body, sendOk, msg)
			// 记录到Email Log
			if sendOk {
				Log("ok " + email)
			} else {
				Log("no " + email)
			}
		} else {
			Log(msg)
		}
	}
	//	}()

	return
}

// 添加邮件日志
func (this *EmailService) AddEmailLog(email, subject, body string, ok bool, msg string) {
	log := info.EmailLog{LogId: db.NewObjectID(), Email: email, Subject: redactEmailSubject(subject), Body: "", Ok: ok, Msg: classifyEmailLogMessage(msg), CreatedTime: time.Now()}
	db.Insert(db.EmailLogs, log)
}

func redactEmailSubject(subject string) string {
	if len(subject) > 128 {
		return subject[:128]
	}
	return subject
}

func classifyEmailLogMessage(msg string) string {
	if msg == "" {
		return ""
	}
	return "delivery_error"
}

// 展示邮件日志

func (this *EmailService) DeleteEmails(ids []string) bool {
	idsO := make([]ObjectID, len(ids))
	for i, id := range ids {
		idsO[i] = db.MustObjectIDFromHex(id)
	}
	db.DeleteAll(db.EmailLogs, bson.M{"_id": bson.M{"$in": idsO}})

	return true
}
func (this *EmailService) ListEmailLogs(pageNumber, pageSize int, sortField string, isAsc bool, email string) (page info.Page, emailLogs []info.EmailLog) {
	emailLogs = []info.EmailLog{}
	skipNum, sortFieldR := parsePageAndSort(pageNumber, pageSize, sortField, isAsc)
	query := bson.M{}
	if email != "" {
		query["Email"] = bson.M{"$regex": bson.Regex{Pattern: ".*?" + email + ".*", Options: "i"}}
	}
	q := db.EmailLogs.Find(query)
	// 总记录数
	count, _ := q.Count()
	// 列表
	q.Sort(sortFieldR).
		Skip(skipNum).
		Limit(pageSize).
		All(&emailLogs)
	page = info.NewPage(pageNumber, pageSize, count, nil)
	return
}
