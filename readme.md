# Debanator

Easily make a debian apt repo from a bunch of `.deb` files.

- You have some deb files
- You want to be able to `apt-get install` them on systems
- You want this to happen automatically (perhaps the debs are on a webdav share, or in
  github releases)

## Status

Proof of concept. Neither fast, efficient, secure, neat, or featureful

## Discussion

- Yes, you could just use dpkg-scanpackages, but you'd have to write some script which
did that, plus gpg and also fetched your packages from wherever they are.

