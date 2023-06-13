package limitcount_test

import (
	"fmt"
	"testing"

	"github.com/hnchenkai/mx-wsgo/limitcount"
	"github.com/hnchenkai/mx-wsgo/wsmessage"
)

func TestAdd(t *testing.T) {
	limitUnit := limitcount.NewLimitCountUnit(nil)
	limitUnit.Init(nil)
	limitUnit.Run()
	defer limitUnit.Close()
	if _, err := limitUnit.MakeConnStatus("axxx1", "1"); err != nil {
		t.FailNow()
	}
	fmt.Println(limitUnit.Status("axxx1"))
	if err := limitUnit.CloseConnStatus("axxx1", "1", wsmessage.LimitAccept); err != nil {
		t.FailNow()
	}
}
