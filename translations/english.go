package translations

var enUsMap = map[string]string{
	"yes": "Y",
}

func (l langauge) EnUS() Translation {
	return Translation{
		Code:   "en_US",
		values: enUsMap,
	}
}

func (l langauge) En() Translation {
	return l.EnUS()
}
