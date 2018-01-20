# Maintainer: Manu Sánchez (Manu343726) <Manu343726.public@gmail.com>

# Tags description:
# =================
#
# Package versioning
# ------------------
#
# VERSION: biicode version of the package, using dot syntax (i.e. 1.9.2).
# VERSION_LABEL: biicode version of the label, using underscores (i.e. 1_9_2).
#
# RELEASE_NUMBER: This is the release number specific to the Arch Linux release. This allows package 
#                 maintainers to make updates to the package’s configure flags, for example. This is 
#                 typically set to 1 for each new upstream software release and incremented for 
#                 intermediate PKGBUILD updates. The variable is not allowed to contain hyphens.
#
# Checksums
# ---------
#
# SUM_32: Checksum (md5) of the 32 bit package.
# SUM_64: Checksum (md5) of the 64 bit package.
# SUM_PI: Checksum (md5) of the RaspberryPi package.
#
# Dependencies
# ------------
# 
# ARCH_DEPS: Dependencies of the package. The names of the packages are usually not the same as their
#            Debian equivalents, should be translated.
#            Those dependencies should be specified using a bash array initializer of the form "'dep1' 'dep2' 'dep3' ... 'depN'"
# 
# BEBIAN_DEPS: Dependencies of the original Debian package. Those should be specified in a list of the form "dep1,dep2,dep3,...,depN"
#
# Package name
# ------------
#
# The name of the debian package that should be downloaded is generated from the pattern "bii_[PKG_PREFIX]_[VERSION_LABEL].deb", where
# VERSION_LABEL is the tag explained above, and PKG_PREFIX is the prefix of the name, which changes depending on the platform.
#
# PKG_PREFIX_32: Name prefix of the 32 bit package.
# PKG_PREFIX_64: Name prefix of the 64 bit package.
# PKG_PREFIX_PI: Name prefix of the RaspBerryPi package.
#


pkgname=biicode
pkgver=3.3
pkgrel=1
pkgdesc="Simple C/C++ file-based dependency manager"
arch=('i686' 'x86_64' 'armv6h')
url="https://www.biicode.com"
license=('unknown')

depends=('cmake>=3.0.2' 'zlib' 'glibc' 'sqlite' 'wget' 'python2-pmw')
options=('!strip')

echo "${CARCH}"

declare -A _package_sums=(["i686"]="9cb1c3f2f00f26b6a8f1474a5935f071"
                          ["x86_64"]="80771adc36a210d0b2776808adccf0d7"
                          ["armv6h"]="3101647100b68a17568fb35623447872")


declare -A _package_prefixes=(["i686"]="ubuntu-32"
                              ["x86_64"]="ubuntu-64"
                              ["armv6h"]="debian-armv6")

_prefix=${_package_prefixes[$CARCH]}
_sum=${_package_sums[$CARCH]}
_package="bii-${_prefix}_3_3.deb"
_source_url="https://s3.amazonaws.com/biibinaries/release/3.3/${_package}"


source=("${_source_url}")                                                                
md5sums=("${_sum}")

noextract=('${_package}')

package()
{
    ar -x ${_package}
    tar -zvxf data.tar.gz -C ${pkgdir}

    chmod 755 "${pkgdir}/usr/"
    chmod 755 "${pkgdir}/usr/share/"
    chmod 755 "${pkgdir}/usr/bin/"
    chmod 755 "${pkgdir}/usr/share/doc/"
}
