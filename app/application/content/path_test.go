package content

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/yangphere/leanote/app/domain"
)

func TestParseStoredPathNormalizesSupportedLegacyPrefixes(t *testing.T) {
	tests := []struct {
		value string
		want  LogicalPath
	}{
		{"files/user/images/a.png", LogicalPath{Kind: RootPrivateFiles, Value: "user/images/a.png"}},
		{`files\user\attachs\a.txt`, LogicalPath{Kind: RootPrivateFiles, Value: "user/attachs/a.txt"}},
		{"public/upload/logo/a.png", LogicalPath{Kind: RootPublicUpload, Value: "logo/a.png"}},
		{"upload/avatar/a.png", LogicalPath{Kind: RootPublicUpload, Value: "avatar/a.png"}},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, err := ParseStoredPath(test.value)
			if err != nil || got != test.want {
				t.Fatalf("ParseStoredPath() = %+v, %v; want %+v", got, err, test.want)
			}
		})
	}
}

func TestLogicalPathsRejectPortableEscapeAndAliasCases(t *testing.T) {
	values := []string{
		"../secret", `..\secret`, "/absolute", `C:\secret`, `C:secret`, `\\server\share\secret`,
		`\\?\C:\secret`, `images\..\secret`, "images//a", "images/./a", "images/a/", "images/NUL.txt",
		"images/com1", "images/a. ", "images/a.", "images/a:b", "images/a\x00b", "images/a\x1fb",
	}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			_, err := ParseLogicalPath(RootPrivateFiles, value)
			var contentErr *Error
			if !errors.As(err, &contentErr) || contentErr.Category != ErrorUnsafePath {
				t.Fatalf("ParseLogicalPath(%q) error = %v; want unsafe_path", value, err)
			}
		})
	}
}

func TestStoredPathsRejectUnknownOrEscapingPrefixes(t *testing.T) {
	for _, value := range []string{"other/a", "files/../secret", `files\..\secret`, `/files/a`, `C:\files\a`} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseStoredPath(value); err == nil {
				t.Fatalf("ParseStoredPath(%q) unexpectedly succeeded", value)
			}
		})
	}
}

func TestStableDestinationBindsIdentityFields(t *testing.T) {
	owner, err := domain.ParseObjectID("507f1f77bcf86cd799439011")
	if err != nil {
		t.Fatal(err)
	}
	identity := AssetIdentity{
		OperationID: "upload:one", Generation: 4, OwnerID: owner, Kind: AssetImage,
		SourceID: "source", DestinationID: "destination", Digest: sha256.Sum256([]byte("image")),
	}
	one, err := StableDestination(identity, RootPrivateFiles, "objects", ".PNG")
	if err != nil {
		t.Fatal(err)
	}
	two, err := StableDestination(identity, RootPrivateFiles, "objects", ".png")
	if err != nil || one != two {
		t.Fatalf("stable destination changed: one=%+v two=%+v err=%v", one, two, err)
	}
	identity.Generation++
	changed, err := StableDestination(identity, RootPrivateFiles, "objects", ".png")
	if err != nil || changed == one {
		t.Fatalf("generation did not change destination: changed=%+v err=%v", changed, err)
	}
}
