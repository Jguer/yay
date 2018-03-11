# Maintainer : Tobias Powalowski <tpowa@archlinux.org>
# Maintainer : Ronald van Haren <ronald.archlinux.org>
# Contributor: Keshav Amburay <(the ddoott ridikulus ddoott rat) (aatt) (gemmaeiil) (ddoott) (ccoomm)>

## "1" to enable IA32-EFI build in Arch x86_64, "0" to disable
_IA32_EFI_IN_ARCH_X64="1"

## "1" to enable EMU build, "0" to disable
_GRUB_EMU_BUILD="0"

_pkgver="2.02"
_GRUB_GIT_TAG="grub-2.02-beta2"

_UNIFONT_VER="6.3.20131217"

[[ "${CARCH}" == "x86_64" ]] && _EFI_ARCH="x86_64"
[[ "${CARCH}" == "i686" ]] && _EFI_ARCH="i386"

[[ "${CARCH}" == "x86_64" ]] && _EMU_ARCH="x86_64"
[[ "${CARCH}" == "i686" ]] && _EMU_ARCH="i386"

pkgname="grub"
pkgdesc="GNU GRand Unified Bootloader (2)"
pkgver=2.02.beta2
pkgrel=5
epoch="1"
url="https://www.gnu.org/software/grub/"
arch=('x86_64' 'i686')
license=('GPL3')
backup=('boot/grub/grub.cfg' 'etc/default/grub' 'etc/grub.d/40_custom')
install="${pkgname}.install"
options=('!makeflags')

conflicts=('grub-common' 'grub-bios' 'grub-emu' "grub-efi-${_EFI_ARCH}" 'grub-legacy')
replaces=('grub-common' 'grub-bios' 'grub-emu' "grub-efi-${_EFI_ARCH}")
provides=('grub-common' 'grub-bios' 'grub-emu' "grub-efi-${_EFI_ARCH}")

makedepends=('git' 'rsync' 'xz' 'freetype2' 'ttf-dejavu' 'python' 'autogen'
             'texinfo' 'help2man' 'gettext' 'device-mapper' 'fuse')
depends=('sh' 'xz' 'gettext' 'device-mapper')
optdepends=('freetype2: For grub-mkfont usage'
            'fuse: For grub-mount usage'
            'dosfstools: For grub-mkrescue FAT FS and EFI support' 
            'efibootmgr: For grub-install EFI support'
            'libisoburn: Provides xorriso for generating grub rescue iso using grub-mkrescue'
            'os-prober: To detect other OSes when generating grub.cfg in BIOS systems'
            'mtools: For grub-mkrescue FAT FS support')

if [[ "${_GRUB_EMU_BUILD}" == "1" ]]; then
    makedepends+=('libusbx' 'sdl')
    optdepends+=('libusbx: For grub-emu USB support'
                 'sdl: For grub-emu SDL support')
fi

source=("grub-${_pkgver}::git+git://git.sv.gnu.org/grub.git#tag=${_GRUB_GIT_TAG}"
        "grub-extras::git+git://git.sv.gnu.org/grub-extras.git#branch=master"
        "http://ftp.gnu.org/gnu/unifont/unifont-${_UNIFONT_VER}/unifont-${_UNIFONT_VER}.bdf.gz"
        "http://ftp.gnu.org/gnu/unifont/unifont-${_UNIFONT_VER}/unifont-${_UNIFONT_VER}.bdf.gz.sig"
        'grub-10_linux-detect-archlinux-initramfs.patch'
	'grub-intel-ucode.patch'
        'grub-add-GRUB_COLOR_variables.patch'
        '60_memtest86+'
        'grub.default'
        'grub.cfg')

md5sums=('SKIP'
         'SKIP'
         '728b7439ac733a7c0d56049adec364c7'
         'SKIP'
         '945527e0de8d384166a4cf23439ae9ee'
	 'a678629bc82c4e70c48d28242036d1d7'
         'e506ae4a9f9f7d1b765febfa84e10d48'
         'be55eabc102f2c60b38ed35c203686d6'
         'a03ffd56324520393bf574cefccb893d'
         'c8b9511586d57d6f2524ae7898397a46')

_pkgver() {
	cd "${srcdir}/grub-${_pkgver}/"
	echo "$(git describe --tags)" | sed -e 's|grub.||g' -e 's|-|\.|g'
}

prepare() {
	
	cd "${srcdir}/grub-${_pkgver}/"
	
	msg "Patch to load Intel microcode"
	patch -Np1 -i "${srcdir}/grub-intel-ucode.patch"
	echo

	msg "Patch to detect of Arch Linux initramfs images by grub-mkconfig"
	patch -Np1 -i "${srcdir}/grub-10_linux-detect-archlinux-initramfs.patch"
	echo
	
	msg "Patch to enable GRUB_COLOR_* variables in grub-mkconfig"
	## Based on http://lists.gnu.org/archive/html/grub-devel/2012-02/msg00021.html
	patch -Np1 -i "${srcdir}/grub-add-GRUB_COLOR_variables.patch"
	echo
	
	msg "Fix DejaVuSans.ttf location so that grub-mkfont can create *.pf2 files for starfield theme"
	sed 's|/usr/share/fonts/dejavu|/usr/share/fonts/dejavu /usr/share/fonts/TTF|g' -i "${srcdir}/grub-${_pkgver}/configure.ac"
	
	msg "Fix mkinitcpio 'rw' FS#36275"
	sed 's| ro | rw |g' -i "${srcdir}/grub-${_pkgver}/util/grub.d/10_linux.in"
	
	msg "Fix OS naming FS#33393"
	sed 's|GNU/Linux|Linux|' -i "${srcdir}/grub-${_pkgver}/util/grub.d/10_linux.in"
	
	# msg "autogen.sh requires python (2/3). since bzr is in makedepends, use python2 and no need to pull python3"
	# sed 's|python |python2 |g' -i "${srcdir}/grub-${_pkgver}/autogen.sh"
	
	msg "Pull in latest language files"
	./linguas.sh
	echo
	
	msg "Remove not working langs which need LC_ALL=C.UTF-8"
	sed -e 's#en@cyrillic en@greek##g' -i "${srcdir}/grub-${_pkgver}/po/LINGUAS"
	
	msg "Avoid problem with unifont during compile of grub, http://savannah.gnu.org/bugs/?40330 and https://bugs.archlinux.org/task/37847"
	cp "${srcdir}/unifont-${_UNIFONT_VER}.bdf" "${srcdir}/grub-${_pkgver}/unifont.bdf"
	
}

_build_grub-common_and_bios() {
	
	msg "Set ARCH dependent variables for bios build"
	if [[ "${CARCH}" == 'x86_64' ]]; then
		_EFIEMU="--enable-efiemu"
	else
		_EFIEMU="--disable-efiemu"
	fi
	
	msg "Copy the source for building the bios part"
	cp -r "${srcdir}/grub-${_pkgver}" "${srcdir}/grub-${_pkgver}-bios"
	cd "${srcdir}/grub-${_pkgver}-bios/"
	
	msg "Add the grub-extra sources for bios build"
	install -d "${srcdir}/grub-${_pkgver}-bios/grub-extras"
	cp -r "${srcdir}/grub-extras/915resolution" "${srcdir}/grub-${_pkgver}-bios/grub-extras/915resolution"
	export GRUB_CONTRIB="${srcdir}/grub-${_pkgver}-bios/grub-extras/"
	
	msg "Unset all compiler FLAGS for bios build"
	unset CFLAGS
	unset CPPFLAGS
	unset CXXFLAGS
	unset LDFLAGS
	unset MAKEFLAGS
	
	cd "${srcdir}/grub-${_pkgver}-bios/"
	
	msg "Run autogen.sh for bios build"
	./autogen.sh
	echo
	
	msg "Run ./configure for bios build"
	./configure \
		--with-platform="pc" \
		--target="i386" \
		"${_EFIEMU}" \
		--enable-mm-debug \
		--enable-nls \
		--enable-device-mapper \
		--enable-cache-stats \
		--enable-boot-time \
		--enable-grub-mkfont \
		--enable-grub-mount \
		--prefix="/usr" \
		--bindir="/usr/bin" \
		--sbindir="/usr/bin" \
		--mandir="/usr/share/man" \
		--infodir="/usr/share/info" \
		--datarootdir="/usr/share" \
		--sysconfdir="/etc" \
	 	--program-prefix="" \
		--with-bootdir="/boot" \
		--with-grubdir="grub" \
		--disable-silent-rules \
		--disable-werror
	echo
	
	msg "Run make for bios build"
	make
	echo
}

_build_grub-efi() {
	
	msg "Copy the source for building the ${_EFI_ARCH} efi part"
	cp -r "${srcdir}/grub-${_pkgver}" "${srcdir}/grub-${_pkgver}-efi-${_EFI_ARCH}"
	cd "${srcdir}/grub-${_pkgver}-efi-${_EFI_ARCH}/"
	
	msg "Unset all compiler FLAGS for ${_EFI_ARCH} efi build"
	unset CFLAGS
	unset CPPFLAGS
	unset CXXFLAGS
	unset LDFLAGS
	unset MAKEFLAGS
	
	cd "${srcdir}/grub-${_pkgver}-efi-${_EFI_ARCH}/"
	
	msg "Run autogen.sh for ${_EFI_ARCH} efi build"
	./autogen.sh
	echo
	
	msg "Run ./configure for ${_EFI_ARCH} efi build"
	./configure \
		--with-platform="efi" \
		--target="${_EFI_ARCH}" \
		--disable-efiemu \
		--enable-mm-debug \
		--enable-nls \
		--enable-device-mapper \
		--enable-cache-stats \
		--enable-boot-time \
		--enable-grub-mkfont \
		--enable-grub-mount \
		--prefix="/usr" \
		--bindir="/usr/bin" \
		--sbindir="/usr/bin" \
		--mandir="/usr/share/man" \
		--infodir="/usr/share/info" \
		--datarootdir="/usr/share" \
		--sysconfdir="/etc" \
		--program-prefix="" \
		--with-bootdir="/boot" \
		--with-grubdir="grub" \
		--disable-silent-rules \
		--disable-werror
	echo
	
	msg "Run make for ${_EFI_ARCH} efi build"
	make
	echo
	
}

_build_grub-emu() {
	
	msg "Copy the source for building the emu part"
	cp -r "${srcdir}/grub-${_pkgver}/" "${srcdir}/grub-${_pkgver}-emu/"
	
	msg "Unset all compiler FLAGS for emu build"
	unset CFLAGS
	unset CPPFLAGS
	unset CXXFLAGS
	unset LDFLAGS
	unset MAKEFLAGS
	
	cd "${srcdir}/grub-${_pkgver}-emu/"
	
	msg "Run autogen.sh for emu build"
	./autogen.sh
	echo
	
	msg "Run ./configure for emu build"
	./configure \
		--with-platform="emu" \
		--target="${_EMU_ARCH}" \
		--enable-mm-debug \
		--enable-nls \
		--enable-device-mapper \
		--enable-cache-stats \
		--enable-grub-mkfont \
		--enable-grub-mount \
		--enable-grub-emu-usb=no \
		--enable-grub-emu-sdl=no \
		--disable-grub-emu-pci \
		--prefix="/usr" \
		--bindir="/usr/bin" \
		--sbindir="/usr/bin" \
		--mandir="/usr/share/man" \
		--infodir="/usr/share/info" \
		--datarootdir="/usr/share" \
		--sysconfdir="/etc" \
		--program-prefix="" \
		--with-bootdir="/boot" \
		--with-grubdir="grub" \
		--disable-silent-rules \
		--disable-werror
	echo
	
	msg "Run make for emu build"
	make
	echo
	
}

build() {
	
	cd "${srcdir}/grub-${_pkgver}/"
	
	msg "Build grub bios stuff"
	_build_grub-common_and_bios
	echo
	
	msg "Build grub ${_EFI_ARCH} efi stuff"
	_build_grub-efi
	echo
	
	if [[ "${CARCH}" == "x86_64" ]] && [[ "${_IA32_EFI_IN_ARCH_X64}" == "1" ]]; then
		msg "Build grub i386 efi stuff"
		_EFI_ARCH="i386" _build_grub-efi
		echo
	fi
	
	if [[ "${_GRUB_EMU_BUILD}" == "1" ]]; then
		msg "Build grub emu stuff"
		_build_grub-emu
		echo
	fi
    
}

_package_grub-common_and_bios() {
	
	cd "${srcdir}/grub-${_pkgver}-bios/"
	
	msg "Run make install for bios build"
	make DESTDIR="${pkgdir}/" bashcompletiondir="/usr/share/bash-completion/completions" install
	echo
	
	msg "Remove gdb debugging related files for bios build"
	rm -f "${pkgdir}/usr/lib/grub/i386-pc"/*.module || true
	rm -f "${pkgdir}/usr/lib/grub/i386-pc"/*.image || true
	rm -f "${pkgdir}/usr/lib/grub/i386-pc"/{kernel.exec,gdb_grub,gmodule.pl} || true
	
	msg "Install extra /etc/grub.d/ files"
	install -D -m0755 "${srcdir}/60_memtest86+" "${pkgdir}/etc/grub.d/60_memtest86+"
	
	msg "Install /etc/default/grub (used by grub-mkconfig)"
	install -D -m0644 "${srcdir}/grub.default" "${pkgdir}/etc/default/grub"
	
	msg "Install grub.cfg for backup array"
	install -D -m0644 "${srcdir}/grub.cfg" "${pkgdir}/boot/grub/grub.cfg"
	
}

_package_grub-efi() {
	
	cd "${srcdir}/grub-${_pkgver}-efi-${_EFI_ARCH}/"
	
	msg "Run make install for ${_EFI_ARCH} efi build"
	make DESTDIR="${pkgdir}/" bashcompletiondir="/usr/share/bash-completion/completions" install
	echo
	
	msg "Remove gdb debugging related files for ${_EFI_ARCH} efi build"
	rm -f "${pkgdir}/usr/lib/grub/${_EFI_ARCH}-efi"/*.module || true
	rm -f "${pkgdir}/usr/lib/grub/${_EFI_ARCH}-efi"/*.image || true
	rm -f "${pkgdir}/usr/lib/grub/${_EFI_ARCH}-efi"/{kernel.exec,gdb_grub,gmodule.pl} || true
	
}

_package_grub-emu() {
	
	cd "${srcdir}/grub-${_pkgver}-emu/"
	
	msg "Run make install for emu build"
	make DESTDIR="${pkgdir}/" bashcompletiondir="/usr/share/bash-completion/completions" install
	echo
	
	msg "Remove gdb debugging related files for emu build"
	rm -f "${pkgdir}/usr/lib/grub/${_EMU_ARCH}-emu"/*.module || true
	rm -f "${pkgdir}/usr/lib/grub/${_EMU_ARCH}-emu"/*.image || true
	rm -f "${pkgdir}/usr/lib/grub/${_EMU_ARCH}-emu"/{kernel.exec,gdb_grub,gmodule.pl} || true
	
}

package() {
	
	cd "${srcdir}/grub-${_pkgver}/"
	
	msg "Package grub ${_EFI_ARCH} efi stuff"
	_package_grub-efi
	
	if [[ "${CARCH}" == "x86_64" ]] && [[ "${_IA32_EFI_IN_ARCH_X64}" == "1" ]]; then
		msg "Package grub i386 efi stuff"
		_EFI_ARCH="i386" _package_grub-efi
		echo
	fi
	
	if [[ "${_GRUB_EMU_BUILD}" == "1" ]]; then
		msg "Package grub emu stuff"
		_package_grub-emu
		echo
	fi
	
	msg "Package grub bios stuff"
	_package_grub-common_and_bios
	
}
