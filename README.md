SCGI reverse proxy transport module for Caddy
===============================================
[![test](https://github.com/Elegant996/scgi-transport/workflows/test.yml/badge.svg?branch=main)](https://github.com/Elegant996/scgi-transport/actions)
[![license](https://img.shields.io/badge/license-Apache%202.0-blue)](https://github.com/Elegant996/scgi-transport/blob/main/LICENSE)

This plugin adds SCGI reverse proxying support to Caddy.

The `scgi` transport module is based on the `fastcgi` transport module available.


SCGI Directive
-----------------------------------------------
To use the `scgi` directive, it must first be added under caddy's global setting:
```
{
  order   scgi after reverse_proxy
}
```

### Syntax ###
```
scgi [<matcher>] <gateways...> {
  root <path>
  split <substrings...>
  env [<key> <value>]
  resolve_root_symlink
  dial_timeout  <duration>
  read_timeout  <duration>
  write_timeout <duration>
  capture_stderr

  <any other reverse_proxy subdirectives...>
}
```

Reverse Proxy
-----------------------------------------------
The `scgi` transport may also be specified under the `reverse_proxy` handler.

### Expanded Form ###
```
route {
  reverse_proxy [<matcher>] <gateway> {
    transport scgi {
      ...
    }
  }
} 
```

Docker
-----------------------------------------------
You may pull a pre-compiled container image of `caddy` embedded with this module through any of the [tagged images](https://github.com/Elegant996/scgi-transport/pkgs/container/scgi-transport) on the GitHub Container Registry or using the `latest` tag below:

```
docker pull ghcr.io/elegant996/scgi-transport:latest
```