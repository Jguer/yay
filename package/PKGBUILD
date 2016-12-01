# Maintainer: Jguer <joaogg3@gmail.com>
pkgname=yay
pkgver=1.60
pkgrel=1
pkgdesc="Yet another yogurt. Pacman wrapper with AUR support written in go."
arch=('i686' 'x86_64')
license=('GPL')
depends=(
  'sudo'
)
makedepends=(
	'go'
	'git'
)

source=(
	"yay::git://github.com/jguer/yay.git#branch=${BRANCH:-master}"
)

md5sums=(
	'SKIP'
)

backup=(
)

pkgver() {
	if [[ "$PKGVER" ]]; then
		echo "$PKGVER"
		return
	fi

	cd "$srcdir/$pkgname"
	local count=$(git rev-list --count HEAD)
	echo "1.${count}"
}

build() {
	cd "$srcdir/$pkgname"

	if [ -L "$srcdir/$pkgname" ]; then
		rm "$srcdir/$pkgname" -rf
		mv "$srcdir/.go/src/$pkgname/" "$srcdir/$pkgname"
	fi

	rm -rf "$srcdir/.go/src"

	mkdir -p "$srcdir/.go/src"

	export GOPATH="$srcdir/.go"

	mv "$srcdir/$pkgname" "$srcdir/.go/src/"

	cd "$srcdir/.go/src/$pkgname/cmd/yay"
	ln -sf "$srcdir/.go/src/$pkgname/cmd/yay" "$srcdir/$pkgname"

	git submodule update --init

	go get -v \
		-gcflags "-trimpath $GOPATH/src" \
		-ldflags="-X main.version=$pkgver"
}

package() {
  #install executable
	find "$srcdir/.go/bin/" -type f -executable | while read filename; do
		install -DT "$filename" "$pkgdir/usr/bin/$(basename $filename)"
	done

	cd "$srcdir/.go/src/$pkgname"

  # Install GLP v3
  mkdir -p "${pkgdir}/usr/share/licenses/${pkgname}"
  install -m644 LICENSE "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"


  # Install zsh completion
  mkdir -p "${pkgdir}/usr/share/zsh/site-functions"
  install -m644 zsh-completion "${pkgdir}/usr/share/zsh/site-functions/_yay"

  # Install fish completion
  mkdir -p "${pkgdir}/usr/share/fish/vendor_completions.d"
install -m644 yay.fish "${pkgdir}/usr/share/fish/vendor_completions.d/yay.fish"
}
