package router

import "testing"

func TestArvPriceShort(t *testing.T) {
	cases := map[int]string{18560000: "18.6M تومان", 850000: "850K تومان", 0: "", 999: "999 تومان"}
	for in, want := range cases {
		if got := arvPriceShort(in); got != want {
			t.Fatalf("%d -> %q want %q", in, got, want)
		}
	}
	if len(arvFallbackSizes()) == 0 || arvFallbackSizes()[0].Price == 0 {
		t.Fatal("fallback sizes must carry prices")
	}
}
