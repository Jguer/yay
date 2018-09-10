package ini

import (
	"strings"
	"io/ioutil"
)

type Callback func(fileName string, line int, section string,
	key string, value string, data interface{}) error

func Parse(ini string, cb Callback, data interface{}) error {
	return parse("", ini, cb, data)
}

func ParseFile(fileName string, cb Callback, data interface{}) error {
	file, err := ioutil.ReadFile(fileName)
	if err != nil {
		return cb(fileName, -1, err.Error(), "", "", data)
	}

	return parse(fileName, string(file), cb, data)
}

func parse(fileName string, ini string, cb Callback, data interface{}) error {
	lines := strings.Split(ini, "\n")
	header := ""

	for n, line := range lines {
		line = strings.TrimSpace(line)

		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			runes := []rune(line)
			header = string(runes[1 : len(runes)-1])
			
			if err := cb(fileName, n, header, "", "", data); err != nil {
				return err
			}
			continue
		}

		key, value := splitPair(line)
		if err := cb(fileName, n, header, key, value, data); err != nil {
			return err
		}
	}

	return nil
}

func splitPair(line string) (string, string) {
	split := strings.SplitN(line, "=", 2)

	key := strings.TrimSpace(split[0])

	if len(split) == 1 {
		return key, ""
	}

	value := strings.TrimSpace(split[1])
	return key, value
}
