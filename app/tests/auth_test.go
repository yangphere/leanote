package tests

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/leanote/leanote/app/db"
	"github.com/leanote/leanote/app/service"
)

func TestAuth(t *testing.T) {
	connection, err := net.DialTimeout("tcp", "127.0.0.1:27017", time.Second)
	if err != nil {
		if os.Getenv("LEANOTE_REQUIRE_MONGO") == "1" {
			t.Fatalf("MongoDB fixture is required at 127.0.0.1:27017: %v", err)
		}
		t.Skipf("MongoDB fixture is unavailable at 127.0.0.1:27017: %v", err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}

	db.Init("mongodb://127.0.0.1:27017/leanote_test", "leanote_test")
	service.InitService()

	_, err = service.AuthS.Login("admin", "abc123")
	if err != nil {
		t.Fatalf("admin fixture authentication failed: %v", err)
	}
}
