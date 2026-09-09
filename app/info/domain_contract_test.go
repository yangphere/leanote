package info

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

func TestRePreservesDynamicJSONWireValues(t *testing.T) {
	var zero Re
	data, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshal zero Re: %v", err)
	}
	if got, want := string(data), `{"Ok":false,"Code":0,"Msg":"","Id":"","List":null,"Item":null}`; got != want {
		t.Fatalf("zero Re = %s, want %s", got, want)
	}

	value := Re{Ok: true, List: []string{}, Item: map[string]interface{}{"count": float64(1)}}
	if err := value.ValidateDynamicValues(); err != nil {
		t.Fatalf("ValidateDynamicValues valid value: %v", err)
	}
	data, err = json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal dynamic Re: %v", err)
	}
	if got, want := string(data), `{"Ok":true,"Code":0,"Msg":"","Id":"","List":[],"Item":{"count":1}}`; got != want {
		t.Fatalf("dynamic Re = %s, want %s", got, want)
	}
}

func TestAPIDTOJSONContractsPreserveIDAndCollectionShapes(t *testing.T) {
	const id = "507f1f77bcf86cd799439011"
	auth := AuthOk{Ok: true}
	parsed, err := domain.ParseObjectID(id)
	if err != nil {
		t.Fatalf("ParseObjectID: %v", err)
	}
	auth.UserId = parsed
	got, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("marshal AuthOk: %v", err)
	}
	if want := `{"Ok":true,"Token":"","UserId":"507f1f77bcf86cd799439011","Email":"","Username":""}`; string(got) != want {
		t.Fatalf("AuthOk JSON = %s, want %s", got, want)
	}

	note := ApiNote{NoteId: id, Tags: nil, Files: nil}
	got, err = json.Marshal(note)
	if err != nil {
		t.Fatalf("marshal nil ApiNote: %v", err)
	}
	if want := `{"NoteId":"507f1f77bcf86cd799439011","NotebookId":"","UserId":"","Title":"","Desc":"","Tags":null,"Abstract":"","Content":"","IsMarkdown":false,"IsBlog":false,"IsTrash":false,"IsDeleted":false,"Usn":0,"Files":null,"CreatedTime":"0001-01-01T00:00:00Z","UpdatedTime":"0001-01-01T00:00:00Z","PublicTime":"0001-01-01T00:00:00Z"}`; string(got) != want {
		t.Fatalf("nil ApiNote JSON = %s, want %s", got, want)
	}

	note.Tags = []string{}
	note.Files = []NoteFile{}
	got, err = json.Marshal(note)
	if err != nil {
		t.Fatalf("marshal empty ApiNote: %v", err)
	}
	if want := `{"NoteId":"507f1f77bcf86cd799439011","NotebookId":"","UserId":"","Title":"","Desc":"","Tags":[],"Abstract":"","Content":"","IsMarkdown":false,"IsBlog":false,"IsTrash":false,"IsDeleted":false,"Usn":0,"Files":[],"CreatedTime":"0001-01-01T00:00:00Z","UpdatedTime":"0001-01-01T00:00:00Z","PublicTime":"0001-01-01T00:00:00Z"}`; string(got) != want {
		t.Fatalf("empty ApiNote JSON = %s, want %s", got, want)
	}
}

func TestThemeInfoRejectsNonJSONValues(t *testing.T) {
	value := Theme{Info: map[string]interface{}{"channel": func() {}}}
	if err := value.ValidateInfo(); err == nil {
		t.Fatal("ValidateInfo accepted non-JSON Theme.Info value")
	}
	if _, err := json.Marshal(value); err == nil {
		t.Fatal("expected non-JSON Theme.Info value to return an error")
	} else if !strings.Contains(err.Error(), "Theme.Info") {
		t.Fatalf("Theme.Info marshal error = %v, want field context", err)
	}
}

func TestReRejectsNonJSONValuesWithFieldContext(t *testing.T) {
	value := Re{List: func() {}}
	if _, err := json.Marshal(value); err == nil {
		t.Fatal("expected non-JSON Re.List value to return an error")
	} else if !strings.Contains(err.Error(), "Re.List") {
		t.Fatalf("Re.List marshal error = %v, want field context", err)
	}
}

func TestPreviouslyMissingPersistenceModelsHaveStableJSONShape(t *testing.T) {
	if got, err := json.Marshal(HasShareNote{}); err != nil {
		t.Fatalf("marshal HasShareNote: %v", err)
	} else if want := `{"HasShareNotebookId":"","UserId":"","ToUserId":"","Seq":0}`; string(got) != want {
		t.Fatalf("HasShareNote JSON = %s, want %s", got, want)
	}
	if got, err := json.Marshal(NoteImage{}); err != nil {
		t.Fatalf("marshal NoteImage: %v", err)
	} else if want := `{"NoteImageId":"","NoteId":"","ImageId":""}`; string(got) != want {
		t.Fatalf("NoteImage JSON = %s, want %s", got, want)
	}
}
