module zombiezen.com/go/bass

go 1.15

require (
	crawshaw.io/sqlite v0.3.3-zombiezen
	github.com/google/go-cmp v0.3.1
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/sys v0.0.0-20201119102817-f84b799fce68
)

replace crawshaw.io/sqlite v0.3.3-zombiezen => github.com/zombiezen/sqlite v0.3.3-0.20201229170853-3aff1a1a78df
