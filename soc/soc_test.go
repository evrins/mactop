package soc

import "testing"

func TestGetSOCInfo(t *testing.T) {
	soc1 := GetSOCInfo()
	t.Log(soc1)
}
