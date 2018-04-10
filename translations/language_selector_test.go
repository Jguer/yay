package translations

import (
	"os"
	"testing"
)

func TestGetTranslationToReturnRightLang(t *testing.T) {
	lang := GetTranslation()
	if foundLang := findLang(); lang.Code != foundLang {
		t.Fatal("Language code is different from the envoirment")
		t.Fatalf("Expected %s but got %s", lang.Code, foundLang)
	}
	if _, exist := lang.values["yes"]; !exist {
		t.Fatal("Language must at least implement confimation text!")
	}
}

func TestFindLangToReturnRightLang(t *testing.T) {
	lang := "en_US.UTF8"
	err := os.Setenv("LANG", lang)
	if err != nil {
		t.Fatal(err.Error())
	}

	langCode := findLang()
	if langCode != lang[:5] {
		t.Fatalf("Expected %s but got %s", langCode, os.Getenv("LANG"))
	}
}
