SCGI reverse proxy transport module for Caddy
===============================================

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
You may download a pre-compiled version of `caddy` embedded with this module through the GitHub Container Registry using any of the below:

```
docker pull ghcr.io/elegant996/scgi-transport:1.0.4-caddy-2.8.4
docker pull ghcr.io/elegant996/scgi-transport:1.0.4
docker pull ghcr.io/elegant996/scgi-transport:latest
```