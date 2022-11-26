# wpactl

## Introduction

Control WPA supplicant through d-bus interface

Call `wpactl --help` to get an overview of possible commands.

Shell (bash) completion is also supported to simplify handling.

To build the application execute `make`. To cross compile the tool for ARM execute `make ARCH=arm`.

## Debian packages

To generate a Debian binary package for the actual platform call the following

`make deb`

To build a package for armv7 machines execute the following

`make deb ARCH=arm`

If you want to use the tool as a normal user the user must be part of the group ´netdev´. This can be done by `adduser username netdev`.
