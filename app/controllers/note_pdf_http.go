package controllers

import "github.com/yangphere/leanote/app/httpserver"

// NotePDFServer retains the legacy /note/toPdf binding until interface-http
// freezes its wire contract. It never authorizes a callback or exposes note
// content; PDF execution belongs exclusively to the content application.
type NotePDFServer struct{}

func NewNotePDFServer(*httpserver.Config) *NotePDFServer {
	return &NotePDFServer{}
}

func (s *NotePDFServer) Register(rs *httpserver.Registry) {
	rs.Register("Note", "ToPdf", nil, s.toPDF)
}

func (s *NotePDFServer) toPDF(c *httpserver.Context) httpserver.Result {
	// Bind and discard legacy inputs; they are not renderer credentials.
	_, _ = c.Params.String("noteId"), c.Params.String("appKey")
	return c.RenderText("no note")
}
