.PHONY: deb clean tag install bin

ARCH ?= $(shell go env GOARCH)
ifeq ($(ARCH),arm)
DEBARCH=armhf
GOARCH = $(ARCH) GOARM=7
CROSS_COMPILE=arm-linux-gnueabihf-
else ifeq ($(ARCH),amd64)
DEBARCH=$(ARCH)
GOARCH=$(ARCH)
CROSS_COMPILE=x86_64-linux-gnu-
else
$(error $(ARCH) is not supported yet)
endif
PV = 0.0.2
PN = wpactl
DESCRIPTION = Control WPA supplicant through d-bus interface
GO_CALL := GOARCH=$(GOARCH) go
DEBIAN_DESTDIR := $(CURDIR)/debuild.d
USR_BIN_DIR = /usr/bin
DEB_USR_BIN_DIR = $(DEBIAN_DESTDIR)$(USR_BIN_DIR)
PKG_DIR = $(CURDIR)/pkgs
PKG_FILE = $(PKG_DIR)/$(PN)_$(PV)+sloppy_$(DEBARCH).deb

bin:
	$(GO_CALL) build

deb: clean bin
	install -D --strip --strip-program=$(CROSS_COMPILE)strip --mode=755 --target-directory=$(DEB_USR_BIN_DIR) $(PN)
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/usr/share/doc/$(PN) README.md LICENSE
	install --mode 644 -D wpactl-completion $(DEBIAN_DESTDIR)/usr/share/bash-completion/completions/wpactl
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/lib/udev/rules.d 94-wpactl.rules
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/lib/systemd/system wpactl@.service
	mkdir $(DEBIAN_DESTDIR)/DEBIAN
	{ \
		echo Package: $(PN); \
		echo Architecture: $(DEBARCH); \
		echo Section: net; \
		echo Priority: optional; \
		echo 'Maintainer: $(DEBFULLNAME) <$(DEBEMAIL)>'; \
		echo Installed-Size: `du --summarize $(DEBIAN_DESTDIR) | cut --fields=1`; \
		echo Depends: libc6, wpasupplicant; \
		echo Version: $(PV)+sloppy; \
		echo Description: $(DESCRIPTION); \
	} > $(DEBIAN_DESTDIR)/DEBIAN/control
	install --directory $(PKG_DIR)
	dpkg-deb --deb-format=2.0 --root-owner-group --build $(DEBIAN_DESTDIR) $(PKG_DIR)
	@echo Package is in directory $(PKG_DIR)

clean:
	rm -r -f $(PN) $(DEBIAN_DESTDIR) $(PKG_DIR)

tag:
	git tag v$(PV)

install: deb
	pkexec apt-get install --reinstall $(PKG_FILE)
