package news

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jguer/yay/v12/pkg/text"
)

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"dc:creator"`
}

func (item *item) print(buildTime time.Time, all, quiet bool) {
	var fd string

	date, err := time.Parse(time.RFC1123Z, item.PubDate)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else {
		fd = text.FormatTime(int(date.Unix()))
		if !all && !buildTime.IsZero() {
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

func PrintNewsFeed(ctx context.Context, client *http.Client, cutOffDate time.Time, bottomUp, all, quiet bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://archlinux.org/feeds/news", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	rssGot := rss{}

	d := xml.NewDecoder(bytes.NewReader(body))
	if err := d.Decode(&rssGot); err != nil {
		return err
	}

	if bottomUp {
		for i := len(rssGot.Channel.Items) - 1; i >= 0; i-- {
			rssGot.Channel.Items[i].print(cutOffDate, all, quiet)
		}
	} else {
		for i := 0; i < len(rssGot.Channel.Items); i++ {
			rssGot.Channel.Items[i].print(cutOffDate, all, quiet)
		}
	}

	return nil
}

// Crude html parsing, good enough for the arch news
// This is only displayed in the terminal so there should be no security
// concerns.
func parseNews(str string) string {
	var (
		buffer       bytes.Buffer
		tagBuffer    bytes.Buffer
		escapeBuffer bytes.Buffer
		inTag        = false
		inEscape     = false
	)

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
