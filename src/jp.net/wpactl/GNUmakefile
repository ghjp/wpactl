.PHONY: all clean tag

ARCH ?= amd64
ifeq ($(ARCH),arm)
	DEBARCH=armhf
	GOARCH = arm GOARM=7
	CROSS_COMPILE=arm-linux-gnueabihf-
else
	DEBARCH=amd64
	GOARCH=$(ARCH)
	CROSS_COMPILE=x86_64-linux-gnu-
endif
PV = 0.0.2
PN = wpactl
DESCRIPTION = Control WPA supplicant through d-bus interface
GO_PATH = $(CURDIR)/../../..
GO_CALL := GOPATH=$(GO_PATH) GOARCH=$(GOARCH) go
DEBIAN_DESTDIR := $(CURDIR)/debuild.d
USR_BIN_DIR = /usr/bin
DEB_USR_BIN_DIR = $(DEBIAN_DESTDIR)$(USR_BIN_DIR)
PKG_DIR = $(CURDIR)/pkgs

all:
	$(MAKE) clean
	$(GO_CALL) build
	install -D --strip --strip-program=$(CROSS_COMPILE)strip --mode=755 --target-directory=$(DEB_USR_BIN_DIR) $(PN)
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/usr/share/doc/$(PN) $(GO_PATH)/README.md $(GO_PATH)/LICENSE
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/etc/bash_completion.d wpactl-completion
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/lib/udev/rules.d 94-wpactl.rules
	install --mode 644 -D --target-directory=$(DEBIAN_DESTDIR)/lib/systemd/system wpactl@.service
	mkdir $(DEBIAN_DESTDIR)/DEBIAN
	{ \
		echo Package: $(PN); \
		echo Architecture: $(DEBARCH); \
		echo Section: net; \
		echo Priority: optional; \
		echo 'Maintainer: $(DEBFULLNAME) <$(DEBEMAIL)>'; \
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
