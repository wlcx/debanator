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

## Contributing
Contributions welcome! You may find the Github mirror at https://github.com/wlcx/debanator easier if you're not already there.


## Usage

`debanator -debpath ./path/to/your/debs -httppass hunter2`

For more, see `-help`.

Then, on the system you want packages on:

- `echo "deb http://debanator:hunter2@<host of debanator>:1612/ stable main`
- `curl http://debanator:hunter2@<host of debanator>:1612/pubkey.gpg | apt-key add -`
- `apt update`
- `apt install your-package`


## Tailscale

Debanator supports listening inside a [Tailscale] tailnet as a ["Virtual Private
Service"][vps]. To enable this mode, generate an auth key in the Tailscale admin panel
and pass it as the `TS_AUTHKEY` env var. You can change the hostname debanator uses with
`-tailscalehostname`.

Note that debanator still respects `-listenaddr`, which given you're inside a tailnet
now you probably just want to set to `:80`.

## Discussion

- Yes, you could just use dpkg-scanpackages, but you'd have to write some script which
did that, plus gpg and also fetched your packages from wherever they are.

[tailscale]: https://tailscale.com
[vps]: https://tailscale.com/blog/tsnet-virtual-private-services/
