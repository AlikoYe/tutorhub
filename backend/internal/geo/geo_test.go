package geo

import "testing"

func TestValidDistrict(t *testing.T) {
	if !ValidDistrict("almaty", "medeu") || !ValidDistrict("almaty", "") || ValidDistrict("almaty", "soho") || ValidDistrict("astana", "medeu") {
		t.Fatal("ValidDistrict is wrong")
	}
}

func TestAllIn(t *testing.T) {
	if !AllIn([]string{"kk", "ru"}, Languages) || AllIn([]string{"kk", "kk"}, Languages) || AllIn([]string{"de"}, Languages) || !AllIn(nil, Languages) {
		t.Fatal("AllIn is wrong")
	}
}
