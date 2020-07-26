package news

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/bradleyjkemp/cupaloy"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

const sampleNews = `<?xml version="1.0" encoding="utf-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel><title>Arch Linux: Recent news updates</title><link>https://www.archlinux.org/news/</link><description>The latest and greatest news from the Arch Linux distribution.</description><atom:link href="https://www.archlinux.org/feeds/news/" rel="self"></atom:link><language>en-us</language><lastBuildDate>Tue, 14 Apr 2020 16:30:32 +0000</lastBuildDate><item><title>zn_poly 0.9.2-2 update requires manual intervention</title><link>https://www.archlinux.org/news/zn_poly-092-2-update-requires-manual-intervention/</link><description>&lt;p&gt;The zn_poly package prior to version 0.9.2-2 was missing a soname link.
This has been fixed in 0.9.2-2, so the upgrade will need to overwrite the
untracked files created by ldconfig. If you get an error&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;zn_poly: /usr/lib/libzn_poly-0.9.so  exists in filesystem
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;when updating, use&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;pacman -Syu --overwrite usr/lib/libzn_poly-0.9.so
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;to perform the upgrade.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Antonio Rojas</dc:creator><pubDate>Tue, 14 Apr 2020 16:30:30 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-04-14:/news/zn_poly-092-2-update-requires-manual-intervention/</guid></item><item><title>nss&gt;=3.51.1-1 and lib32-nss&gt;=3.51.1-1 updates require manual intervention</title><link>https://www.archlinux.org/news/nss3511-1-and-lib32-nss3511-1-updates-require-manual-intervention/</link><description>&lt;p&gt;The nss and lib32-nss packages prior to version 3.51.1-1 were missing a soname link each. This has been fixed in 3.51.1-1, so the upgrade will need to overwrite the untracked files created by ldconfig. If you get any of these errors&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;nss: /usr/lib/p11-kit-trust.so exists in filesystem
lib32-nss: /usr/lib32/p11-kit-trust.so exists in filesystem
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;when updating, use&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;pacman -Syu --overwrite /usr/lib\*/p11-kit-trust.so
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;to perform the upgrade.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Jan Alexander Steffens</dc:creator><pubDate>Mon, 13 Apr 2020 00:35:58 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-04-13:/news/nss3511-1-and-lib32-nss3511-1-updates-require-manual-intervention/</guid></item><item><title>hplip 3.20.3-2 update requires manual intervention</title><link>https://www.archlinux.org/news/hplip-3203-2-update-requires-manual-intervention/</link><description>&lt;p&gt;The hplip package prior to version 3.20.3-2 was missing the compiled
python modules. This has been fixed in 3.20.3-2, so the upgrade will
need to overwrite the untracked pyc files that were created. If you get errors
such as these&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;hplip: /usr/share/hplip/base/__pycache__/__init__.cpython-38.pyc exists in filesystem
hplip: /usr/share/hplip/base/__pycache__/avahi.cpython-38.pyc exists in filesystem
hplip: /usr/share/hplip/base/__pycache__/codes.cpython-38.pyc exists in filesystem
...many more...
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;when updating, use&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;pacman -Suy --overwrite /usr/share/hplip/\*
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;to perform the upgrade.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Andreas Radke</dc:creator><pubDate>Thu, 19 Mar 2020 06:53:30 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-03-19:/news/hplip-3203-2-update-requires-manual-intervention/</guid></item><item><title>firewalld&gt;=0.8.1-2 update requires manual intervention</title><link>https://www.archlinux.org/news/firewalld081-2-update-requires-manual-intervention/</link><description>&lt;p&gt;The firewalld package prior to version 0.8.1-2 was missing the compiled python modules. This has been fixed in 0.8.1-2, so the upgrade will need to overwrite the untracked pyc files created. If you get errors like these&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;firewalld: /usr/lib/python3.8/site-packages/firewall/__pycache__/__init__.cpython-38.pyc exists in filesystem
firewalld: /usr/lib/python3.8/site-packages/firewall/__pycache__/client.cpython-38.pyc exists in filesystem
firewalld: /usr/lib/python3.8/site-packages/firewall/__pycache__/dbus_utils.cpython-38.pyc exists in filesystem
...many more...
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;when updating, use&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;pacman -Suy --overwrite /usr/lib/python3.8/site-packages/firewall/\*
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;to perform the upgrade.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Jan Alexander Steffens</dc:creator><pubDate>Sun, 01 Mar 2020 16:36:48 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-03-01:/news/firewalld081-2-update-requires-manual-intervention/</guid></item><item><title>The Future of the Arch Linux Project Leader</title><link>https://www.archlinux.org/news/the-future-of-the-arch-linux-project-leader/</link><description>&lt;p&gt;Hello everyone,&lt;/p&gt;
&lt;p&gt;Some of you may know me from the days when I was much more involved in Arch, but most of you probably just know me as a name on the website. I’ve been with Arch for some time, taking the leadership of this beast over from Judd back in 2007. But, as these things often go, my involvement has slid down to minimal levels over time. It’s high time that changes.&lt;/p&gt;
&lt;p&gt;Arch Linux needs involved leadership to make hard decisions and direct the project where it needs to go. And I am not in a position to do this.&lt;/p&gt;
&lt;p&gt;In a team effort, the Arch Linux staff devised a new process for determining future leaders. From now on, leaders will be elected by the staff for a term length of two years. Details of this new process can be found &lt;a href="https://wiki.archlinux.org/index.php/DeveloperWiki:Project_Leader"&gt;here&lt;/a&gt;&lt;/p&gt;
&lt;p&gt;In the first official vote with Levente Polyak (anthraxx), Gaetan Bisson (vesath), Giancarlo Razzolini (grazzolini), and Sven-Hendrik Haase (svenstaro) as candidates, and through 58 verified votes, a winner was chosen:&lt;/p&gt;
&lt;p&gt;&lt;strong&gt;Levente Polyak (anthraxx) will be taking over the reins of this ship. Congratulations!&lt;/strong&gt;&lt;/p&gt;
&lt;p&gt;&lt;em&gt;Thanks for everything over all these years,&lt;br /&gt;
Aaron Griffin (phrakture)&lt;/em&gt;&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Aaron Griffin</dc:creator><pubDate>Mon, 24 Feb 2020 15:56:28 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-02-24:/news/the-future-of-the-arch-linux-project-leader/</guid></item><item><title>Planet Arch Linux migration</title><link>https://www.archlinux.org/news/planet-arch-linux-migration/</link><description>&lt;p&gt;The software behind planet.archlinux.org was implemented in Python 2 and is no longer maintained upstream. This functionality has now been implemented in archlinux.org's archweb backend which is actively maintained but offers a slightly different experience.&lt;/p&gt;
&lt;p&gt;The most notable changes are the offered feeds and the feed location. Archweb only offers an Atom feed which is located at &lt;a href="https://archlinux.org/feeds/planet"&gt;here&lt;/a&gt;.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Jelle van der Waa</dc:creator><pubDate>Sat, 22 Feb 2020 22:43:00 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-02-22:/news/planet-arch-linux-migration/</guid></item><item><title>sshd needs restarting after upgrading to openssh-8.2p1</title><link>https://www.archlinux.org/news/sshd-needs-restarting-after-upgrading-to-openssh-82p1/</link><description>&lt;p&gt;After upgrading to openssh-8.2p1, the existing SSH daemon will be unable to accept new connections. (See &lt;a href="https://bugs.archlinux.org/task/65517"&gt;FS#65517&lt;/a&gt;.) When upgrading remote hosts, please make sure to restart the SSH daemon using &lt;code&gt;systemctl restart sshd&lt;/code&gt; right after running &lt;code&gt;pacman -Syu&lt;/code&gt;. If you are upgrading to openssh-8.2p1-3 or higher, this restart will happen automatically.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Gaetan Bisson</dc:creator><pubDate>Mon, 17 Feb 2020 01:35:04 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-02-17:/news/sshd-needs-restarting-after-upgrading-to-openssh-82p1/</guid></item><item><title>rsync compatibility</title><link>https://www.archlinux.org/news/rsync-compatibility/</link><description>&lt;p&gt;Our &lt;code&gt;rsync&lt;/code&gt; package was shipped with bundled &lt;code&gt;zlib&lt;/code&gt; to provide compatibility
with the old-style &lt;code&gt;--compress&lt;/code&gt; option up to version 3.1.0. Version 3.1.1 was
released on 2014-06-22 and is shipped by all major distributions now.&lt;/p&gt;
&lt;p&gt;So we decided to finally drop the bundled library and ship a package with
system &lt;code&gt;zlib&lt;/code&gt;. This also fixes security issues, actual ones and in future. Go
and blame those running old versions if you encounter errors with &lt;code&gt;rsync
3.1.3-3&lt;/code&gt;.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Christian Hesse</dc:creator><pubDate>Wed, 15 Jan 2020 20:14:43 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-01-15:/news/rsync-compatibility/</guid></item><item><title>Now using Zstandard instead of xz for package compression</title><link>https://www.archlinux.org/news/now-using-zstandard-instead-of-xz-for-package-compression/</link><description>&lt;p&gt;As announced on the &lt;a href="https://lists.archlinux.org/pipermail/arch-dev-public/2019-December/029752.html"&gt;mailing list&lt;/a&gt;, on Friday, Dec 27 2019, our package compression scheme has changed from xz (.pkg.tar.xz) to &lt;a href="https://lists.archlinux.org/pipermail/arch-dev-public/2019-December/029778.html"&gt;zstd (.pkg.tar.zst)&lt;/a&gt;.&lt;/p&gt;
&lt;p&gt;zstd and xz trade blows in their compression ratio. Recompressing all packages to zstd with our options yields a total ~0.8% increase in package size on all of our packages combined, but the decompression time for all packages saw a ~1300% speedup.&lt;/p&gt;
&lt;p&gt;We already have more than 545 zstd-compressed packages in our repositories, and as packages get updated more will keep rolling in. We have not found any user-facing issues as of yet, so things appear to be working.&lt;/p&gt;
&lt;p&gt;As a packager, you will automatically start building .pkg.tar.zst packages if you are using the latest version of devtools (&amp;gt;= 20191227).&lt;br /&gt;
As an end-user no manual intervention is required, assuming that you have read and followed the news post &lt;a href="https://www.archlinux.org/news/required-update-to-recent-libarchive/"&gt;from late last year&lt;/a&gt;.&lt;/p&gt;
&lt;p&gt;If you nevertheless haven't updated libarchive since 2018, all hope is not lost! Binary builds of pacman-static are available from Eli Schwartz' &lt;a href="https://wiki.archlinux.org/index.php/Unofficial_user_repositories#eschwartz"&gt;personal repository&lt;/a&gt; (or direct link to &lt;a href="https://pkgbuild.com/~eschwartz/repo/x86_64-extracted/"&gt;binary&lt;/a&gt;), signed with their Trusted User keys, with which you can perform the update.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Robin Broda</dc:creator><pubDate>Sat, 04 Jan 2020 20:35:55 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2020-01-04:/news/now-using-zstandard-instead-of-xz-for-package-compression/</guid></item><item><title>Xorg cleanup requires manual intervention</title><link>https://www.archlinux.org/news/xorg-cleanup-requires-manual-intervention/</link><description>&lt;p&gt;In the process of &lt;a href="https://bugs.archlinux.org/task/64892"&gt;Xorg cleanup&lt;/a&gt; the update requires manual
intervention when you hit this message:&lt;/p&gt;
&lt;pre&gt;&lt;code&gt;:: installing xorgproto (2019.2-2) breaks dependency 'inputproto' required by lib32-libxi
:: installing xorgproto (2019.2-2) breaks dependency 'dmxproto' required by libdmx
:: installing xorgproto (2019.2-2) breaks dependency 'xf86dgaproto' required by libxxf86dga
:: installing xorgproto (2019.2-2) breaks dependency 'xf86miscproto' required by libxxf86misc
&lt;/code&gt;&lt;/pre&gt;
&lt;p&gt;when updating, use: &lt;code&gt;pacman -Rdd libdmx libxxf86dga libxxf86misc &amp;amp;&amp;amp; pacman -Syu&lt;/code&gt; to perform the upgrade.&lt;/p&gt;</description><dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">Andreas Radke</dc:creator><pubDate>Fri, 20 Dec 2019 13:37:40 +0000</pubDate><guid isPermaLink="false">tag:www.archlinux.org,2019-12-20:/news/xorg-cleanup-requires-manual-intervention/</guid></item></channel></rss>
`

func TestPrintNewsFeed(t *testing.T) {
	layout := "2006-01-02"
	str := "2020-04-13"
	lastNewsTime, _ := time.Parse(layout, str)

	type args struct {
		cutOffDate time.Time
		sortMode   int
		all        bool
		quiet      bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "all-verbose", args: args{cutOffDate: time.Now(), all: true, quiet: false}, wantErr: false},
		{name: "all-quiet", args: args{cutOffDate: lastNewsTime, all: true, quiet: true}, wantErr: false},
		{name: "latest-quiet", args: args{cutOffDate: lastNewsTime, all: false, quiet: true}, wantErr: false},
		{name: "latest-quiet-topdown", args: args{sortMode: 1, cutOffDate: lastNewsTime, all: false, quiet: true}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer gock.Off()

			gock.New("https://archlinux.org").
				Get("/feeds/news").
				Reply(200).
				BodyString(sampleNews)
			rescueStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			err := PrintNewsFeed(tt.args.cutOffDate, tt.args.sortMode, tt.args.all, tt.args.quiet)
			assert.NoError(t, err)

			w.Close()
			out, _ := ioutil.ReadAll(r)
			cupaloy.SnapshotT(t, out)
			os.Stdout = rescueStdout
		})
	}
}
