# wpactl

## Introduction

Control WPA supplicant through d-bus interface

Call `wpactl --help` to get an overview of possible commands.

Shell (bash) completion is also supported to simplify handling.

## Debian packages

To generate a Debian binary package for the actual platform call the following

`make -C src/jp.net/wpactl`

To build a package for armv7 machines execute the following

`make -C src/jp.net/wpactl ARCH=arm`

If you want to use the tool as a normal user the user must be part of the group ´netdev´. This can be done by `adduser username netdev`.
