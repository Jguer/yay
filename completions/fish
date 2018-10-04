# vim:fdm=marker foldlevel=0 tabstop=2 shiftwidth=2 filetype=fish
# Original Author for pacman: Giorgio Lando <patroclo7@gmail.com>
# Updated for yay by jguer

set -l progname yay
complete -e -c $progname
complete -c $progname -f

set -l listinstalled "(pacman -Q | string replace ' ' \t)"
# This might be an issue if another package manager is also installed (e.g. for containers)
set -l listall "(yay -Pc)"
set -l listrepos "(__fish_print_pacman_repos)"
set -l listgroups "(pacman -Sg)\t'Package Group'"
set -l listpacman "(__fish_print_packages)"
set -l noopt 'not __fish_contains_opt -s Y -s G -s V -s P -s S -s D -s Q -s R -s U -s T -s F database query sync remove upgrade deptest files'
set -l database '__fish_contains_opt -s D database'
set -l getpkgbuild '__fish_contains_opt -s G getpkgbuild'
set -l show '__fish_contains_opt -s P show'
set -l query '__fish_contains_opt -s Q query'
set -l remove '__fish_contains_opt -s R remove'
set -l sync '__fish_contains_opt -s S sync'
set -l upgrade '__fish_contains_opt -s U upgrade'
set -l files '__fish_contains_opt -s F files'
set -l yayspecific '__fish_contains_opt -s Y yay'

# HACK: We only need these two to coerce fish to stop file completion and complete options
# complete -c $progname -n $noopt -a "-D" -d "Modify the package database"
# complete -c $progname -n $noopt -a "-Q" -d "Query the package database"

# Primary operations
complete -c $progname -s D -f -l database -n $noopt -d 'Modify the package database'
complete -c $progname -s F -f -l files -n $noopt -d 'Query the files database'
complete -c $progname -s G -f -l getpkgbuild -n $noopt -d 'Get PKGBUILD from ABS or AUR'
complete -c $progname -s P -f -l show -n $noopt -d 'Print information'
complete -c $progname -s Q -f -l query -n $noopt -d 'Query the package database'
complete -c $progname -s R -f -l remove -n $noopt -d 'Remove packages from the system'
complete -c $progname -s S -f -l sync -n $noopt -d 'Synchronize packages'
complete -c $progname -s T -f -l deptest -n $noopt -d 'Check if dependencies are installed'
complete -c $progname -s U -f -l upgrade -n $noopt -d 'Upgrade or add a local package'
complete -c $progname -s Y -f -l yay -n $noopt -d 'Yay specific operations'
complete -c $progname -s V -f -l version -n $noopt -d 'Display version and exit'
complete -c $progname -s h -f -l help -n $noopt -d 'Display help'

# General options
# Only offer these once a command has been given so they get prominent display
complete -c $progname -n "not $noopt" -s a -l aur -d 'Assume targets are from the repositories'
complete -c $progname -n "not $noopt" -l repo -d 'Assume targets are from the AUR'

complete -c $progname -n "not $noopt" -s b -l aururl -d 'Set an alternative AUR URL' -f
complete -c $progname -n "not $noopt" -s b -l dbpath -d 'Alternative database location' -xa '(__fish_complete_directories)'
complete -c $progname -n "not $noopt" -s r -l root -d 'Alternative installation root'
complete -c $progname -n "not $noopt" -s v -l verbose -d 'Output more status messages'
complete -c $progname -n "not $noopt" -s h -l help -d 'Display syntax for the given operation'
complete -c $progname -n "not $noopt" -l arch -d 'Alternate architecture' -f
complete -c $progname -n "not $noopt" -l cachedir -d 'Alternative package cache location'
complete -c $progname -n "not $noopt" -l color -d 'Colorize the output'
complete -c $progname -n "not $noopt" -l config -d 'Alternate config file'
complete -c $progname -n "not $noopt" -l debug -d 'Display debug messages' -f
complete -c $progname -n "not $noopt" -l gpgdir -d 'GPG directory to verify signatures'
complete -c $progname -n "not $noopt" -l hookdir -d 'Hook file directory'
complete -c $progname -n "not $noopt" -l logfile -d 'Specify alternative log file'

complete -c $progname -n "not $noopt" -l noconfirm -d 'Bypass any question' -f
complete -c $progname -n "not $noopt" -l topdown -d 'Shows repository packages first and then aur' -f
complete -c $progname -n "not $noopt" -l bottomup -d 'Shows aur packages first and then repository' -f
complete -c $progname -n "not $noopt" -l devel -d 'Check -git/-svn/-hg development version' -f
complete -c $progname -n "not $noopt" -l nodevel -d 'Disable development version checking' -f
complete -c $progname -n "not $noopt" -l cleanafter -d 'Clean package sources after successful build' -f
complete -c $progname -n "not $noopt" -l nocleanafter -d 'Disable package sources cleaning' -f
complete -c $progname -n "not $noopt" -l timeupdate -d 'Check package modification date and version' -f
complete -c $progname -n "not $noopt" -l notimeupdate -d 'Check only package version change' -f

complete -c $progname -n "not $noopt" -l save -d 'Save current arguments to yay permanent configuration' -f
complete -c $progname -n "not $noopt" -l mflags -d 'Pass the following options to makepkg' -f
complete -c $progname -n "not $noopt" -l gpgflags -d 'Pass the following options to gpg' -f
complete -c $progname -n "not $noopt" -l buildir -d 'Specify the build directory' -f
complete -c $progname -n "not $noopt" -l editor -d 'Editor to use' -f
complete -c $progname -n "not $noopt" -l editorflags -d 'Editor flags to use' -f
complete -c $progname -n "not $noopt" -l makepkg -d 'Makepkg command to use' -f
complete -c $progname -n "not $noopt" -l pacman -d 'Pacman command to use' -f
complete -c $progname -n "not $noopt" -l tar -d 'Tar command to use' -f
complete -c $progname -n "not $noopt" -l git -d 'Git command to use' -f
complete -c $progname -n "not $noopt" -l gpg -d 'Gpg command to use' -f
complete -c $progname -n "not $noopt" -l requestsplitn -d 'Max amount of packages to query per AUR request' -f
complete -c $progname -n "not $noopt" -l sudoloop -d 'Loop sudo calls in the background to avoid timeout' -f
complete -c $progname -n "not $noopt" -l nosudoloop -d 'Do not loop sudo calls in the background' -f
complete -c $progname -n "not $noopt" -l redownload -d 'Redownload PKGBUILD of package even if up-to-date' -f
complete -c $progname -n "not $noopt" -l noredownload -d 'Do not redownload up-to-date PKGBUILDs' -f
complete -c $progname -n "not $noopt" -l redownloadall -d 'Redownload PKGBUILD of package and deps even if up-to-date' -f
complete -c $progname -n "not $noopt" -l rebuild -d 'Always build target packages' -f
complete -c $progname -n "not $noopt" -l rebuildall -d 'Always build all AUR packages' -f
complete -c $progname -n "not $noopt" -l rebuildtree -d 'Always build all AUR packages even if installed' -f
complete -c $progname -n "not $noopt" -l norebuild -d 'Skip package build if in cache and up to date' -f

complete -c $progname -n "not $noopt" -l sortby -d 'Sort AUR results by a specific field during search' -f

complete -c $progname -n "not $noopt" -l answerclean -d 'Set a predetermined answer for the clean build menu' -f
complete -c $progname -n "not $noopt" -l answeredit -d 'Set a predetermined answer for the edit pkgbuild menu' -f
complete -c $progname -n "not $noopt" -l answerupgrade -d 'Set a predetermined answer for the upgrade menu' -f
complete -c $progname -n "not $noopt" -l noanswerclean -d 'Unset the answer for the clean build menu' -f
complete -c $progname -n "not $noopt" -l noansweredit -d 'Unset the answer for the edit pkgbuild menu' -f
complete -c $progname -n "not $noopt" -l noanswerupgrade -d 'Unset the answer for the upgrade menu' -f

complete -c $progname -n "not $noopt" -l cleanmenu -d 'Give the option to clean build PKGBUILDS' -f
complete -c $progname -n "not $noopt" -l diffmenu -d 'Give the option to show diffs for build files' -f
complete -c $progname -n "not $noopt" -l editmenu -d 'Give the option to edit/view PKGBUILDS' -f
complete -c $progname -n "not $noopt" -l upgrademenu -d 'Show a detailed list of updates with the option to skip any' -f
complete -c $progname -n "not $noopt" -l nocleanmenu -d 'Do not clean build PKGBUILDS' -f
complete -c $progname -n "not $noopt" -l nodiffmenu -d 'Do not show diffs for build files' -f
complete -c $progname -n "not $noopt" -l noeditmenu -d 'Do not edit/view PKGBUILDS' -f
complete -c $progname -n "not $noopt" -l noupgrademenu -d 'Do not show the upgrade menu' -f


complete -c $progname -n "not $noopt" -l provides -d 'Look for matching providers when searching for packages'
complete -c $progname -n "not $noopt" -l noprovides -d 'Just look for packages by pkgname'
complete -c $progname -n "not $noopt" -l pgpfetch -d 'Prompt to import PGP keys from PKGBUILDs'
complete -c $progname -n "not $noopt" -l nopgpfetch -d 'Do not prompt to import PGP keys'

# Post V7.887
complete -c $progname -n "not $noopt" -l useask -d 'Automatically resolve conflicts using pacmans ask flag'
complete -c $progname -n "not $noopt" -l nouseask -d 'Confirm conflicts manually during the install'
complete -c $progname -n "not $noopt" -l combinedupgrade -d 'Refresh then perform the repo and AUR upgrade together'
complete -c $progname -n "not $noopt" -l nocombinedupgrade -d 'Perform the repo upgrade and AUR upgrade separately'

#Post V8.976
complete -c $progname -n "not $noopt" -l nomakepkgconf -d 'Use default makepkg.conf'
complete -c $progname -n "not $noopt" -l makepkgconf -d 'Use custom makepkg.conf location'
complete -c $progname -n "not $noopt" -l removemake -d 'Remove make deps after install'
complete -c $progname -n "not $noopt" -l askremovemake -d 'Ask to remove make deps after install'
complete -c $progname -n "not $noopt" -l noremovemake -d 'Do not remove make deps after install'
complete -c $progname -n "not $noopt" -l completioninterval -d 'Refresh interval for completion cache'

# Yay options
complete -c $progname -n $yayspecific -s c -l clean -d 'Remove unneeded dependencies' -f
complete -c $progname -n $yayspecific -l gendb -d 'Generate development package DB' -f

# Show options
complete -c $progname -n $show -s d -l defaultconfig -d 'Print default yay configuration' -f
complete -c $progname -n $show -s g -l currentconfig -d 'Print current yay configuration' -f
complete -c $progname -n $show -s s -l stats -d 'Display system package statistics' -f
complete -c $progname -n $show -s w -l news -d 'Print arch news'
complete -c $progname -n $show -s q -l quiet -d 'Do not print news description'

# Getpkgbuild options
complete -c $progname -n $getpkgbuild -s f -l force -d 'Force download for existing tar packages' -f

# Transaction options (sync, remove, upgrade)
for condition in sync remove upgrade
    complete -c $progname -n $$condition -s d -l nodeps -d 'Skip [all] dependency checks' -f
    complete -c $progname -n $$condition -l dbonly -d 'Modify database entry only' -f
    complete -c $progname -n $$condition -l noprogressbar -d 'Do not display progress bar' -f
    complete -c $progname -n $$condition -l noscriptlet -d 'Do not execute install script' -f
    complete -c $progname -n $$condition -s p -l print -d 'Dry run, only print targets' -f
    complete -c $progname -n $$condition -l print-format -x -d 'Specify printf-like format' -f
end

# Database and upgrade options (database, sync, upgrade)
for condition in database sync upgrade
    complete -c $progname -n $$condition -l asdeps -d 'Mark PACKAGE as dependency' -f
    complete -c $progname -n $$condition -l asexplicit -d 'Mark PACKAGE as explicitly installed' -f
end

# Upgrade options (sync, upgrade)
for condition in sync upgrade
    complete -c $progname -n $$condition -l force -d 'Bypass file conflict checks' -f
    complete -c $progname -n $$condition -l ignore -d 'Ignore upgrade of PACKAGE' -xa "$listinstalled" -f
    complete -c $progname -n $$condition -l ignoregroup -d 'Ignore upgrade of GROUP' -xa "$listgroups" -f
    complete -c $progname -n $$condition -l needed -d 'Do not reinstall up-to-date targets' -f
    complete -c $progname -n $$condition -l recursive -d 'Recursively reinstall all dependencies' -f
end

# Query and sync options
for condition in query sync
    complete -c $progname -n $$condition -s g -l groups -d 'Display all packages in GROUP' -xa "$listgroups" -f
    complete -c $progname -n $$condition -s i -l info -d 'Display information on PACKAGE' -f
    complete -c $progname -n $$condition -s q -l quiet -d 'Show less information' -f
    complete -c $progname -n $$condition -s s -l search -r -d 'Search packages for regexp' -f
end

# Get PKGBUILD options
complete -c $progname -n "$getpkgbuild" -xa "$listall"

# Query options
complete -c $progname -n $query -s c -l changelog -d 'View the change log of PACKAGE' -f
complete -c $progname -n $query -s d -l deps -d 'List only non-explicit packages (dependencies)' -f
complete -c $progname -n $query -s e -l explicit -d 'List only explicitly installed packages' -f
complete -c $progname -n $query -s k -l check -d 'Check if all files owned by PACKAGE are present' -f
complete -c $progname -n $query -s l -l list -d 'List all files owned by PACKAGE' -f
complete -c $progname -n $query -s m -l foreign -d 'List all packages not in the database' -f
complete -c $progname -n $query -s o -l owns -r -d 'Search for the package that owns FILE' -xa '' -f
complete -c $progname -n $query -s p -l file -d 'Apply the query to a package file, not package' -xa '' -f
complete -c $progname -n $query -s t -l unrequired -d 'List only unrequired packages' -f
complete -c $progname -n $query -s u -l upgrades -d 'List only out-of-date packages' -f
complete -c $progname -n "$query" -d 'Installed package' -xa $listinstalled -f

# Remove options
complete -c $progname -n $remove -s c -l cascade -d 'Also remove packages depending on PACKAGE' -f
complete -c $progname -n $remove -s n -l nosave -d 'Ignore file backup designations' -f
complete -c $progname -n $remove -s s -l recursive -d 'Also remove dependencies of PACKAGE' -f
complete -c $progname -n $remove -s u -l unneeded -d 'Only remove targets not required by PACKAGE' -f
complete -c $progname -n "$remove" -d 'Installed package' -xa $listinstalled -f

# Sync options
complete -c $progname -n $sync -s c -l clean -d 'Remove [all] packages from cache'
complete -c $progname -n $sync -s l -l list -xa "$listrepos" -d 'List all packages in REPOSITORY'
complete -c $progname -n "$sync; and not __fish_contains_opt -s u sysupgrade" -s u -l sysupgrade -d 'Upgrade all packages that are out of date'
complete -c $progname -n "$sync; and __fish_contains_opt -s u sysupgrade" -s u -l sysupgrade -d 'Also downgrade packages'
complete -c $progname -n $sync -s w -l downloadonly -d 'Only download the target packages'
complete -c $progname -n $sync -s y -l refresh -d 'Download fresh copy of the package list'
complete -c $progname -n "$sync" -xa "$listall $listgroups"

# Database options
set -l has_db_opt '__fish_contains_opt asdeps asexplicit'
complete -c $progname -n "$database; and not $has_db_opt" -xa --asdeps -d 'Mark PACKAGE as dependency'
complete -c $progname -n "$database; and not $has_db_opt" -xa --asexplicit -d 'Mark PACKAGE as explicitly installed'
complete -c $progname -n "$database; and not $has_db_opt" -s k -l check -d 'Check database validity'
complete -c $progname -n "$has_db_opt; and $database" -xa "$listinstalled"

# File options - since pacman 5
set -l has_file_opt '__fish_contains_opt list search -s l -s s'
complete -c $progname -n "$files; and not $has_file_opt" -xa --list -d 'List files owned by given packages'
complete -c $progname -n "$files; and not $has_file_opt" -xa -l -d 'List files owned by given packages'
complete -c $progname -n "$files; and not $has_file_opt" -xa --search -d 'Search packages for matching files'
complete -c $progname -n "$files; and not $has_file_opt" -xa -s -d 'Search packages for matching files'
complete -c $progname -n "$files" -s y -l refresh -d 'Refresh the files database' -f
complete -c $progname -n "$files" -s l -l list -d 'List files owned by given packages' -xa $listpacman
complete -c $progname -n "$files" -s s -l search -d 'Search packages for matching files'
complete -c $progname -n "$files" -s o -l owns -d 'Search for packages that include the given files'
complete -c $progname -n "$files" -s q -l quiet -d 'Show less information' -f
complete -c $progname -n "$files" -l machinereadable -d 'Show in machine readable format: repo\0pkgname\0pkgver\0path\n' -f

# Upgrade options
complete -c $progname -n "$upgrade" -xa '(__fish_complete_suffix pkg.tar.xz)' -d 'Package file'
complete -c $progname -n "$upgrade" -xa '(__fish_complete_suffix pkg.tar.gz)' -d 'Package file'
complete -c $progname -n "$upgrade" -xa '(__fish_complete_suffix pkg.tar.lzo)' -d 'Package file'
complete -c $progname -n "$upgrade" -xa '(__fish_complete_suffix pkg.tar)' -d 'Package file'
