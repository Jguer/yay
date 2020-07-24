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

	"github.com/Jguer/yay/v10/pkg/query"
	"github.com/Jguer/yay/v10/pkg/settings"
	"github.com/Jguer/yay/v10/pkg/text"
)

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"dc:creator"`
}

func (item *item) print(buildTime time.Time, double, quiet bool) {
	var fd string
	date, err := time.Parse(time.RFC1123Z, item.PubDate)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		fd = text.FormatTime(int(date.Unix()))
		if !double && !buildTime.IsZero() {
			if buildTime.After(date) {
				return
			}
		}
	}

	fmt.Println(text.Bold(text.Magenta(fd)), text.Bold(strings.TrimSpace(item.Title)))

	if !quiet {
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

func PrintNewsFeed(alpmHandle *alpm.Handle, sortMode string, double, quiet bool) error {
	resp, err := http.Get("https://archlinux.org/feeds/news")
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	rssGot := rss{}

	d := xml.NewDecoder(bytes.NewReader(body))
	err = d.Decode(&rssGot)
	if err != nil {
		return err
	}

	buildTime, err := lastBuildTime(alpmHandle)
	if err != nil {
		return err
	}

	if sortMode == settings.BottomUp {
		for i := len(rssGot.Channel.Items) - 1; i >= 0; i-- {
			rssGot.Channel.Items[i].print(buildTime, double, quiet)
		}
	} else {
		for i := 0; i < len(rssGot.Channel.Items); i++ {
			rssGot.Channel.Items[i].print(buildTime, double, quiet)
		}
	}

	return nil
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
					buffer.WriteString(text.CyanCode)
				case "/code":
					buffer.WriteString(text.ResetCode)
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

	buffer.WriteString(text.ResetCode)
	return buffer.String()
}
