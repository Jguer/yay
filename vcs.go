package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

// branch contains the information of a repository branch
type branch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

type branches []branch

// Info contains the last commit sha of a repo
type Info struct {
	Package string `json:"pkgname"`
	URL     string `json:"url"`
	SHA     string `json:"sha"`
}

type infos []Info

// Repo contains information about the repository
type repo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"owner"`
	Private          bool        `json:"private"`
	HTMLURL          string      `json:"html_url"`
	Description      string      `json:"description"`
	Fork             bool        `json:"fork"`
	URL              string      `json:"url"`
	ForksURL         string      `json:"forks_url"`
	KeysURL          string      `json:"keys_url"`
	CollaboratorsURL string      `json:"collaborators_url"`
	TeamsURL         string      `json:"teams_url"`
	HooksURL         string      `json:"hooks_url"`
	IssueEventsURL   string      `json:"issue_events_url"`
	EventsURL        string      `json:"events_url"`
	AssigneesURL     string      `json:"assignees_url"`
	BranchesURL      string      `json:"branches_url"`
	TagsURL          string      `json:"tags_url"`
	BlobsURL         string      `json:"blobs_url"`
	GitTagsURL       string      `json:"git_tags_url"`
	GitRefsURL       string      `json:"git_refs_url"`
	TreesURL         string      `json:"trees_url"`
	StatusesURL      string      `json:"statuses_url"`
	LanguagesURL     string      `json:"languages_url"`
	StargazersURL    string      `json:"stargazers_url"`
	ContributorsURL  string      `json:"contributors_url"`
	SubscribersURL   string      `json:"subscribers_url"`
	SubscriptionURL  string      `json:"subscription_url"`
	CommitsURL       string      `json:"commits_url"`
	GitCommitsURL    string      `json:"git_commits_url"`
	CommentsURL      string      `json:"comments_url"`
	IssueCommentURL  string      `json:"issue_comment_url"`
	ContentsURL      string      `json:"contents_url"`
	CompareURL       string      `json:"compare_url"`
	MergesURL        string      `json:"merges_url"`
	ArchiveURL       string      `json:"archive_url"`
	DownloadsURL     string      `json:"downloads_url"`
	IssuesURL        string      `json:"issues_url"`
	PullsURL         string      `json:"pulls_url"`
	MilestonesURL    string      `json:"milestones_url"`
	NotificationsURL string      `json:"notifications_url"`
	LabelsURL        string      `json:"labels_url"`
	ReleasesURL      string      `json:"releases_url"`
	DeploymentsURL   string      `json:"deployments_url"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	PushedAt         time.Time   `json:"pushed_at"`
	GitURL           string      `json:"git_url"`
	SSHURL           string      `json:"ssh_url"`
	CloneURL         string      `json:"clone_url"`
	SvnURL           string      `json:"svn_url"`
	Homepage         string      `json:"homepage"`
	Size             int         `json:"size"`
	StargazersCount  int         `json:"stargazers_count"`
	WatchersCount    int         `json:"watchers_count"`
	Language         string      `json:"language"`
	HasIssues        bool        `json:"has_issues"`
	HasProjects      bool        `json:"has_projects"`
	HasDownloads     bool        `json:"has_downloads"`
	HasWiki          bool        `json:"has_wiki"`
	HasPages         bool        `json:"has_pages"`
	ForksCount       int         `json:"forks_count"`
	MirrorURL        interface{} `json:"mirror_url"`
	Archived         bool        `json:"archived"`
	OpenIssuesCount  int         `json:"open_issues_count"`
	License          struct {
		Key    string      `json:"key"`
		Name   string      `json:"name"`
		SpdxID interface{} `json:"spdx_id"`
		URL    interface{} `json:"url"`
	} `json:"license"`
	Forks         int    `json:"forks"`
	OpenIssues    int    `json:"open_issues"`
	Watchers      int    `json:"watchers"`
	DefaultBranch string `json:"default_branch"`
	Organization  struct {
		Login             string `json:"login"`
		ID                int    `json:"id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		SiteAdmin         bool   `json:"site_admin"`
	} `json:"organization"`
	NetworkCount     int `json:"network_count"`
	SubscribersCount int `json:"subscribers_count"`
}

// createDevelDB forces yay to create a DB of the existing development packages
func createDevelDB() error {
	_, _, _, remoteNames, err := filterPackages()
	if err != nil {
		return err
	}

	config.NoConfirm = true
	arguments := makeArguments()
	arguments.addArg("gendb")
	arguments.addTarget(remoteNames...)
	err = install(arguments)
	return err
}

// parseSource returns owner and repo from source
func parseSource(source string) (owner string, repo string) {
	if !(strings.Contains(source, "git://") ||
		strings.Contains(source, ".git") ||
		strings.Contains(source, "git+https://")) {
		return
	}
	split := strings.Split(source, "github.com/")
	if len(split) > 1 {
		secondSplit := strings.Split(split[1], "/")
		if len(secondSplit) > 1 {
			owner = secondSplit[0]
			thirdSplit := strings.Split(secondSplit[1], ".git")
			if len(thirdSplit) > 0 {
				repo = thirdSplit[0]
			}
		}
	}
	return
}

func (info *Info) needsUpdate() bool {
	var newRepo repo
	var newBranches branches
	if strings.HasSuffix(info.URL, "/branches") {
		info.URL = info.URL[:len(info.URL)-9]
	}
	infoResp, infoErr := http.Get(info.URL)
	if infoErr != nil {
		fmt.Println(infoErr)
		return false
	}
	defer infoResp.Body.Close()

	infoBody, _ := ioutil.ReadAll(infoResp.Body)
	var err = json.Unmarshal(infoBody, &newRepo)
	if err != nil {
		fmt.Printf("Cannot update '%v'\nError: %v\nStatus code: %v\nBody: %v\n",
			info.Package, err, infoResp.StatusCode, string(infoBody))
		return false
	}

	defaultBranch := newRepo.DefaultBranch
	branchesURL := info.URL + "/branches"

	branchResp, branchErr := http.Get(branchesURL)
	if branchErr != nil {
		fmt.Println(branchErr)
		return false
	}
	defer branchResp.Body.Close()

	branchBody, _ := ioutil.ReadAll(branchResp.Body)
	err = json.Unmarshal(branchBody, &newBranches)
	if err != nil {
		fmt.Printf("Cannot update '%v'\nError: %v\nStatus code: %v\nBody: %v\n",
			info.Package, err, branchResp.StatusCode, string(branchBody))
		return false
	}

	for _, e := range newBranches {
		if e.Name == defaultBranch {
			return e.Commit.SHA != info.SHA
		}
	}
	return false
}

func inStore(pkgName string) *Info {
	for i, e := range savedInfo {
		if pkgName == e.Package {
			return &savedInfo[i]
		}
	}
	return nil
}

// branchInfo updates saved information
func branchInfo(pkgName string, owner string, repoName string) (err error) {
	updated = true
	var newRepo repo
	var newBranches branches
	url := "https://api.github.com/repos/" + owner + "/" + repoName
	repoResp, err := http.Get(url)
	if err != nil {
		return
	}
	defer repoResp.Body.Close()

	_ = json.NewDecoder(repoResp.Body).Decode(&newRepo)
	defaultBranch := newRepo.DefaultBranch
	branchesUrl := url + "/branches"

	branchResp, err := http.Get(branchesUrl)
	if err != nil {
		return
	}
	defer branchResp.Body.Close()

	_ = json.NewDecoder(branchResp.Body).Decode(&newBranches)

	packinfo := inStore(pkgName)

	for _, e := range newBranches {
		if e.Name == defaultBranch {
			if packinfo != nil {
				packinfo.Package = pkgName
				packinfo.URL = url
				packinfo.SHA = e.Commit.SHA
			} else {
				savedInfo = append(savedInfo, Info{Package: pkgName, URL: url, SHA: e.Commit.SHA})
			}
		}
	}

	return
}

func saveVCSInfo() error {
	marshalledinfo, err := json.MarshalIndent(savedInfo, "", "\t")
	if err != nil || string(marshalledinfo) == "null" {
		return err
	}
	in, err := os.OpenFile(vcsFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = in.Write(marshalledinfo)
	if err != nil {
		return err
	}
	err = in.Sync()
	return err
}
