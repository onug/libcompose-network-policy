
package nethooks

import (
	"testing"
)

func TestGetUserId(t *testing.T) {
	userId, err := getSelfId()
	if err != nil {
		t.Fatalf("error getting self user id: %s", err)
	}
	t.Logf("got user id : %s", userId)
}
