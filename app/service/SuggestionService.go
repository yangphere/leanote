package service

import (
	"github.com/yangphere/leanote/app/db"
	"github.com/yangphere/leanote/app/info"
	//	. "github.com/yangphere/leanote/app/lea"
	//	"time"
	//	"sort"
)

type SuggestionService struct {
}

// 得到某博客具体信息
func (this *SuggestionService) AddSuggestion(suggestion info.Suggestion) bool {
	if suggestion.Id.IsZero() {
		suggestion.Id = db.NewObjectID()
	}
	return db.Insert(db.Suggestions, suggestion)
}
