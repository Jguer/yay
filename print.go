package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	rpc "github.com/mikkeloscar/aur"
)

const arrow = "==>"
const smallArrow = " ->"

func (warnings *aurWarnings) print() {
	if len(warnings.Missing) > 0 {
		fmt.Print(bold(yellow(smallArrow)) + " Missing AUR Packages:")
		for _, name := range warnings.Missing {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.Orphans) > 0 {
		fmt.Print(bold(yellow(smallArrow)) + " Orphaned AUR Packages:")
		for _, name := range warnings.Orphans {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

	if len(warnings.OutOfDate) > 0 {
		fmt.Print(bold(yellow(smallArrow)) + " Out Of Date AUR Packages:")
		for _, name := range warnings.OutOfDate {
			fmt.Print("  " + cyan(name))
		}
		fmt.Println()
	}

}

// human method returns results in human readable format.
func human(size int64) string {
	floatsize := float32(size)
	units := [...]string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei", "Zi", "Yi"}
	for _, unit := range units {
		if floatsize < 1024 {
			return fmt.Sprintf("%.1f %sB", floatsize, unit)
		}
		floatsize /= 1024
	}
	return fmt.Sprintf("%d%s", size, "B")
}

// PrintSearch handles printing search results in a given format
func (q aurQuery) printSearch(start int) {
	localDb, _ := alpmHandle.LocalDb()

	for i, res := range q {
		var toprint string
		if config.SearchMode == numberMenu {
			if config.SortMode == bottomUp {
				toprint += magenta(strconv.Itoa(len(q)+start-i-1) + " ")
			} else {
				toprint += magenta(strconv.Itoa(start+i) + " ")
			}
		} else if config.SearchMode == minimal {
			fmt.Println(res.Name)
			continue
		}

		toprint += bold(colourHash("aur")) + "/" + bold(res.Name) +
			" " + cyan(res.Version) +
			bold(" (+"+strconv.Itoa(res.NumVotes)) +
			" " + bold(strconv.FormatFloat(res.Popularity, 'f', 2, 64)+"%) ")

		if res.Maintainer == "" {
			toprint += bold(red("(Orphaned)")) + " "
		}

		if res.OutOfDate != 0 {
			toprint += bold(red("(Out-of-date "+formatTime(res.OutOfDate)+")")) + " "
		}

		if pkg, err := localDb.PkgByName(res.Name); err == nil {
			if pkg.Version() != res.Version {
				toprint += bold(green("(Installed: " + pkg.Version() + ")"))
			} else {
				toprint += bold(green("(Installed)"))
			}
		}
		toprint += "\n    " + res.Description
		fmt.Println(toprint)
	}
}

// PrintSearch receives a RepoSearch type and outputs pretty text.
func (s repoQuery) printSearch() {
	for i, res := range s {
		var toprint string
		if config.SearchMode == numberMenu {
			if config.SortMode == bottomUp {
				toprint += magenta(strconv.Itoa(len(s)-i) + " ")
			} else {
				toprint += magenta(strconv.Itoa(i+1) + " ")
			}
		} else if config.SearchMode == minimal {
			fmt.Println(res.Name())
			continue
		}

		toprint += bold(colourHash(res.DB().Name())) + "/" + bold(res.Name()) +
			" " + cyan(res.Version()) +
			bold(" ("+human(res.Size())+
				" "+human(res.ISize())+") ")

		if len(res.Groups().Slice()) != 0 {
			toprint += fmt.Sprint(res.Groups().Slice(), " ")
		}

		localDb, err := alpmHandle.LocalDb()
		if err == nil {
			if pkg, err := localDb.PkgByName(res.Name()); err == nil {
				if pkg.Version() != res.Version() {
					toprint += bold(green("(Installed: " + pkg.Version() + ")"))
				} else {
					toprint += bold(green("(Installed)"))
				}
			}
		}

		toprint += "\n    " + res.Description()
		fmt.Println(toprint)
	}
}

// Pretty print a set of packages from the same package base.
// Packages foo and bar from a pkgbase named base would print like so:
// base (foo bar)
func (base Base) String() string {
	pkg := base[0]
	str := pkg.PackageBase
	if len(base) > 1 || pkg.PackageBase != pkg.Name {
		str2 := " ("
		for _, split := range base {
			str2 += split.Name + " "
		}
		str2 = str2[:len(str2)-1] + ")"

		str += str2
	}

	return str
}

func (u upgrade) StylizedNameWithRepository() string {
	return bold(colourHash(u.Repository)) + "/" + bold(u.Name)
}

// Print prints the details of the packages to upgrade.
func (u upSlice) print() {
	longestName, longestVersion := 0, 0
	for _, pack := range u {
		packNameLen := len(pack.StylizedNameWithRepository())
		version, _ := getVersionDiff(pack.LocalVersion, pack.RemoteVersion)
		packVersionLen := len(version)
		longestName = max(packNameLen, longestName)
		longestVersion = max(packVersionLen, longestVersion)
	}

	namePadding := fmt.Sprintf("%%-%ds  ", longestName)
	versionPadding := fmt.Sprintf("%%-%ds", longestVersion)
	numberPadding := fmt.Sprintf("%%%dd  ", len(fmt.Sprintf("%v", len(u))))

	for k, i := range u {
		left, right := getVersionDiff(i.LocalVersion, i.RemoteVersion)

		fmt.Print(magenta(fmt.Sprintf(numberPadding, len(u)-k)))

		fmt.Printf(namePadding, i.StylizedNameWithRepository())

		fmt.Printf("%s -> %s\n", fmt.Sprintf(versionPadding, left), right)
	}
}

func printDownloads(repoName string, length int, packages string) {
	if length < 1 {
		return
	}

	repoInfo := bold(blue(
		"[" + repoName + ": " + strconv.Itoa(length) + "]"))
	fmt.Println(repoInfo + cyan(packages))
}

func printInfoValue(str, value string) {
	if value == "" {
		value = "None"
	}

	fmt.Printf(bold("%-16s%s")+" %s\n", str, ":", value)
}

// PrintInfo prints package info like pacman -Si.
func PrintInfo(a *rpc.Pkg) {
	printInfoValue("Repository", "aur")
	printInfoValue("Name", a.Name)
	printInfoValue("Keywords", strings.Join(a.Keywords, "  "))
	printInfoValue("Version", a.Version)
	printInfoValue("Description", a.Description)
	printInfoValue("URL", a.URL)
	printInfoValue("AUR URL", config.AURURL+"/packages/"+a.Name)
	printInfoValue("Groups", strings.Join(a.Groups, "  "))
	printInfoValue("Licenses", strings.Join(a.License, "  "))
	printInfoValue("Provides", strings.Join(a.Provides, "  "))
	printInfoValue("Depends On", strings.Join(a.Depends, "  "))
	printInfoValue("Make Deps", strings.Join(a.MakeDepends, "  "))
	printInfoValue("Check Deps", strings.Join(a.CheckDepends, "  "))
	printInfoValue("Optional Deps", strings.Join(a.OptDepends, "  "))
	printInfoValue("Conflicts With", strings.Join(a.Conflicts, "  "))
	printInfoValue("Maintainer", a.Maintainer)
	printInfoValue("Votes", fmt.Sprintf("%d", a.NumVotes))
	printInfoValue("Popularity", fmt.Sprintf("%f", a.Popularity))
	printInfoValue("First Submitted", formatTime(a.FirstSubmitted))
	printInfoValue("Last Modified", formatTime(a.LastModified))

	if a.OutOfDate != 0 {
		printInfoValue("Out-of-date", "Yes ["+formatTime(a.OutOfDate)+"]")
	} else {
		printInfoValue("Out-of-date", "No")
	}

	if cmdArgs.existsDouble("i") {
		printInfoValue("ID", fmt.Sprintf("%d", a.ID))
		printInfoValue("Package Base ID", fmt.Sprintf("%d", a.PackageBaseID))
		printInfoValue("Package Base", a.PackageBase)
		printInfoValue("Snapshot URL", config.AURURL+a.URLPath)
	}

	fmt.Println()
}

// BiggestPackages prints the name of the ten biggest packages in the system.
func biggestPackages() {
	localDb, err := alpmHandle.LocalDb()
	if err != nil {
		return
	}

	pkgCache := localDb.PkgCache()
	pkgS := pkgCache.SortBySize().Slice()

	if len(pkgS) < 10 {
		return
	}

	for i := 0; i < 10; i++ {
		fmt.Println(bold(pkgS[i].Name()) + ": " + cyan(human(pkgS[i].ISize())))
	}
	// Could implement size here as well, but we just want the general idea
}

// localStatistics prints installed packages statistics.
func localStatistics() error {
	info, err := statistics()
	if err != nil {
		return err
	}

	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	fmt.Printf(bold("Yay version v%s\n"), version)
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Total installed packages: ")) + cyan(strconv.Itoa(info.Totaln)))
	fmt.Println(bold(green("Total foreign installed packages: ")) + cyan(strconv.Itoa(len(remoteNames))))
	fmt.Println(bold(green("Explicitly installed packages: ")) + cyan(strconv.Itoa(info.Expln)))
	fmt.Println(bold(green("Total Size occupied by packages: ")) + cyan(human(info.TotalSize)))
	fmt.Println(bold(cyan("===========================================")))
	fmt.Println(bold(green("Ten biggest packages:")))
	biggestPackages()
	fmt.Println(bold(cyan("===========================================")))

	aurInfoPrint(remoteNames)

	return nil
}

//TODO: Make it less hacky
func printNumberOfUpdates() error {
	//todo
	warnings := &aurWarnings{}
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	aurUp, repoUp, err := upList(warnings)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}
	fmt.Println(len(aurUp) + len(repoUp))

	return nil
}

//TODO: Make it less hacky
func printUpdateList(parser *arguments) error {
	targets := sliceToStringSet(parser.targets)
	warnings := &aurWarnings{}
	old := os.Stdout // keep backup of the real stdout
	os.Stdout = nil
	_, _, localNames, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	aurUp, repoUp, err := upList(warnings)
	os.Stdout = old // restoring the real stdout
	if err != nil {
		return err
	}

	noTargets := len(targets) == 0

	if !parser.existsArg("m", "foreign") {
		for _, pkg := range repoUp {
			if noTargets || targets.get(pkg.Name) {
				if parser.existsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	if !parser.existsArg("n", "native") {
		for _, pkg := range aurUp {
			if noTargets || targets.get(pkg.Name) {
				if parser.existsArg("q", "quiet") {
					fmt.Printf("%s\n", pkg.Name)
				} else {
					fmt.Printf("%s %s -> %s\n", bold(pkg.Name), green(pkg.LocalVersion), green(pkg.RemoteVersion))
				}
				delete(targets, pkg.Name)
			}
		}
	}

	missing := false

outer:
	for pkg := range targets {
		for _, name := range localNames {
			if name == pkg {
				continue outer
			}
		}

		for _, name := range remoteNames {
			if name == pkg {
				continue outer
			}
		}

		fmt.Println(red(bold("error:")), "package '"+pkg+"' was not found")
		missing = true
	}

	if missing {
		return fmt.Errorf("")
	}

	return nil
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Creator     string `xml:"dc:creator"`
}

func (item item) print(buildTime time.Time) {
	var fd string
	date, err := time.Parse(time.RFC1123Z, item.PubDate)

	if err != nil {
		fmt.Println(err)
	} else {
		fd = formatTime(int(date.Unix()))
		if _, double, _ := cmdArgs.getArg("news", "w"); !double && !buildTime.IsZero() {
			if buildTime.After(date) {
				return
			}
		}
	}

	fmt.Println(bold(magenta(fd)), bold(strings.TrimSpace(item.Title)))
	//fmt.Println(strings.TrimSpace(item.Link))

	if !cmdArgs.existsArg("q", "quiet") {
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

func printNewsFeed() error {
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

	buildTime, err := lastBuildTime()
	if err != nil {
		return err
	}

	if config.SortMode == bottomUp {
		for i := len(rss.Channel.Items) - 1; i >= 0; i-- {
			rss.Channel.Items[i].print(buildTime)
		}
	} else {
		for i := 0; i < len(rss.Channel.Items); i++ {
			rss.Channel.Items[i].print(buildTime)
		}
	}

	return nil
}

// Formats a unix timestamp to ISO 8601 date (yyyy-mm-dd)
func formatTime(i int) string {
	t := time.Unix(int64(i), 0)
	return t.Format("2006-01-02")
}

const (
	redCode     = "\x1b[31m"
	greenCode   = "\x1b[32m"
	yellowCode  = "\x1b[33m"
	blueCode    = "\x1b[34m"
	magentaCode = "\x1b[35m"
	cyanCode    = "\x1b[36m"
	boldCode    = "\x1b[1m"

	resetCode = "\x1b[0m"
)

func stylize(startCode, in string) string {
	if useColor {
		return startCode + in + resetCode
	}

	return in
}

func red(in string) string {
	return stylize(redCode, in)
}

func green(in string) string {
	return stylize(greenCode, in)
}

func yellow(in string) string {
	return stylize(yellowCode, in)
}

func blue(in string) string {
	return stylize(blueCode, in)
}

func cyan(in string) string {
	return stylize(cyanCode, in)
}

func magenta(in string) string {
	return stylize(magentaCode, in)
}

func bold(in string) string {
	return stylize(boldCode, in)
}

// Colours text using a hashing algorithm. The same text will always produce the
// same colour while different text will produce a different colour.
func colourHash(name string) (output string) {
	if !useColor {
		return name
	}
	var hash uint = 5381
	for i := 0; i < len(name); i++ {
		hash = uint(name[i]) + ((hash << 5) + (hash))
	}
	return fmt.Sprintf("\x1b[%dm%s\x1b[0m", hash%6+31, name)
}

func providerMenu(dep string, providers providers) *rpc.Pkg {
	size := providers.Len()

	fmt.Print(bold(cyan(":: ")))
	str := bold(fmt.Sprintf(bold("There are %d providers available for %s:"), size, dep))

	size = 1
	str += bold(cyan("\n:: ")) + bold("Repository AUR\n    ")

	for _, pkg := range providers.Pkgs {
		str += fmt.Sprintf("%d) %s ", size, pkg.Name)
		size++
	}

	fmt.Println(str)

	for {
		fmt.Print("\nEnter a number (default=1): ")

		if config.NoConfirm {
			fmt.Println("1")
			return providers.Pkgs[0]
		}

		reader := bufio.NewReader(os.Stdin)
		numberBuf, overflow, err := reader.ReadLine()

		if err != nil {
			fmt.Println(err)
			break
		}

		if overflow {
			fmt.Println("Input too long")
			continue
		}

		if string(numberBuf) == "" {
			return providers.Pkgs[0]
		}

		num, err := strconv.Atoi(string(numberBuf))
		if err != nil {
			fmt.Printf("%s invalid number: %s\n", red("error:"), string(numberBuf))
			continue
		}

		if num < 1 || num > size {
			fmt.Printf("%s invalid value: %d is not between %d and %d\n", red("error:"), num, 1, size)
			continue
		}

		return providers.Pkgs[num-1]
	}

	return nil
}
