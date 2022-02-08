package ids

import (
	"fmt"
	"testing"
	"time"
)

func Test_NewULID(t *testing.T) {

	// create 10 ULIDs quickly; the first few bytes should be the same
	// and the last few bytes should be completely random
	var ulids [10]string
	for i := 0; i < 10; i++ {
		ulids[i] = NewULID()
		time.Sleep(1*time.Millisecond + 1)
		fmt.Println(ulids[i])
	}

}
