Changelog
===============================================

# v1.0.2

* Added support for Caddy 2.7.x
* Updated comments

# v1.0.1

* Removed unused names from return variables
* Updated go, caddy and dependencies
* Updated `README.md`

# v1.0.0

* Optimized buffer usage through pools
* Reader/writer functions separated
* Remove unusable support for stderr output

# v0.7.1

* Corrected variable names

# v0.7.0

* Added support for stderr output

# v0.6.0

* Added support for raw responses

# v0.5.0

* Added `root`, `split` and `resolve_root_symlink` syntax to `scgi` directive
* Ensure `SCRIPT_NAME` takes precedence over `PATH_INFO` when no `split` is provided
* Ensure `PATH_INFO` has a leading slash for compliance with RFC3875
* Restored previously removed headers
* Updated dependencies to use `caddy` v2.5.1

# v0.4.0

* Cleaned up code regarding `root`
* Removed reference to empty `MatcherSetsRaw`

This release corrects an issue with the `scgi` directive which was previously being skipped due to the empty matcher.

# v0.3.2

* Added support performing pre-check requests
* Modified module order recommendations in `README.md`

# v0.3.1

* Added `go.mod` and `go.sum`

# v0.3.0

* Added support for Caddy 2.4/2.5
* Updated `dispenser`
* Switched to `zapcore` for environment variables
* Added default dial timeout
* Removed `ioutil` and `path` dependencies

# v0.2.0

* Added support for `PostForm` and `PostFile`
* Added connection timeout checks
* Removed unused subdirectives
* Modified included headers (i.e. server port)
* Updated usage documentation

# v0.1.0

Initial release
