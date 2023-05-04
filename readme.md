# Debanator

Easily make a debian apt repo from a bunch of `.deb` files.

- You have some deb files
- You want to be able to `apt-get install` them on systems
- You want this to happen automatically (perhaps the debs are on a webdav share, or in
  github releases)

## Status

Proof of concept. Neither fast, efficient, secure, neat, or featureful. Don't use in
production unless you are really sure you know what you're doing and even then prepare
to have your laundry eaten.


## Usage

`debanator -debpath ./path/to/your/debs -httppass hunter2`

For more, see `-help`.

Then, on the system you want packages on:

- `echo "deb http://debanator:hunter2@<host of debanator>:1612/ stable main`
- `curl http://debanator:hunter2@<host of debanator>:1612/pubkey.gpg | apt-key add -`
- `apt update`
- `apt install your-package`

## Discussion

- Yes, you could just use dpkg-scanpackages, but you'd have to write some script which
did that, plus gpg and also fetched your packages from wherever they are.

