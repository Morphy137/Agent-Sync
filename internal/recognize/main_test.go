package recognize

import (
	"os"
	"testing"

	"agentsync/internal/testenv"
)

func TestMain(m *testing.M) {
	os.Exit(testenv.Run(m))
}
