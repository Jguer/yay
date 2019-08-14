package news

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jguer/go-alpm"
	"github.com/Jguer/yay/v10/pkg/lookup/query"
	"github.com/Jguer/yay/v10/pkg/runtime"
	"github.com/Jguer/yay/v10/pkg/text"
	"github.com/Jguer/yay/v10/pkg/types"
)

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"dc:creator"`
}

func (item item) print(cmdArgs *types.Arguments, buildTime time.Time) {
	var fd string
	date, err := time.Parse(time.RFC1123Z, item.PubDate)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		fd = text.FormatTime(int(date.Unix()))
		if _, double, _ := cmdArgs.GetArg("news", "w"); !double && !buildTime.IsZero() {
			if buildTime.After(date) {
				return
			}
		}
	}

	fmt.Println(text.Bold(text.Magenta(fd)), text.Bold(strings.TrimSpace(item.Title)))
	//fmt.Println(strings.TrimSpace(item.Link))

	if !cmdArgs.ExistsArg("q", "quiet") {
		desc := strings.TrimSpace(parseNews(item.Description))
		fmt.Println(desc)
	}
}

type channel struct {
	Title         string `xml:"title"`
	Link          string `xml:"link"`
	Description   string `xml:"description"`
	Language      string `xml:"language"`
	Lastbuilddate string `xml:"lastbuilddate"`
	Items         []item `xml:"item"`
}

type rss struct {
	Channel channel `xml:"channel"`
}

func lastBuildTime(alpmHandle *alpm.Handle) (time.Time, error) {
	var lastTime time.Time

	pkgs, _, _, _, err := query.FilterPackages(alpmHandle)
	if err != nil {
		return lastTime, err
	}

	for _, pkg := range pkgs {
		thisTime := pkg.BuildDate()
		if thisTime.After(lastTime) {
			lastTime = thisTime
		}
	}

	return lastTime, nil
}

func PrintFeed(cmdArgs *types.Arguments, alpmHandle *alpm.Handle, sortMode int) error {
	resp, err := http.Get("https://archlinux.org/feeds/news")
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	rss := rss{}

	d := xml.NewDecoder(bytes.NewReader(body))
	err = d.Decode(&rss)
	if err != nil {
		return err
	}

	buildTime, err := lastBuildTime(alpmHandle)
	if err != nil {
		return err
	}

	if sortMode == runtime.BottomUp {
		for i := len(rss.Channel.Items) - 1; i >= 0; i-- {
			rss.Channel.Items[i].print(cmdArgs, buildTime)
		}
	} else {
		for i := 0; i < len(rss.Channel.Items); i++ {
			rss.Channel.Items[i].print(cmdArgs, buildTime)
		}
	}

	return nil
}

// Crude html parsing, good enough for the arch news
// This is only displayed in the terminal so there should be no security
// concerns
func parseNews(str string) string {
	var buffer bytes.Buffer
	var tagBuffer bytes.Buffer
	var escapeBuffer bytes.Buffer
	inTag := false
	inEscape := false

	for _, char := range str {
		if inTag {
			if char == '>' {
				inTag = false
				switch tagBuffer.String() {
				case "code":
					buffer.WriteString("\x1b[36m") // Cyan Code to fix with text package
				case "/code":
					buffer.WriteString("\x1b[0m") // Reset Code to fix with text package
				case "/p":
					buffer.WriteRune('\n')
				}

				continue
			}

			tagBuffer.WriteRune(char)
			continue
		}

		if inEscape {
			if char == ';' {
				inEscape = false
				escapeBuffer.WriteRune(char)
				s := html.UnescapeString(escapeBuffer.String())
				buffer.WriteString(s)
				continue
			}

			escapeBuffer.WriteRune(char)
			continue
		}

		if char == '<' {
			inTag = true
			tagBuffer.Reset()
			continue
		}

		if char == '&' {
			inEscape = true
			escapeBuffer.Reset()
			escapeBuffer.WriteRune(char)
			continue
		}

		buffer.WriteRune(char)
	}

	buffer.WriteString("\x1b[0m")
	return buffer.String()
}
