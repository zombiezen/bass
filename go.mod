module zombiezen.com/go/bass

go 1.16

require (
	crawshaw.io/sqlite v0.3.3-0.20201229170853-3aff1a1a78df
	github.com/google/go-cmp v0.3.1
	github.com/spf13/cobra v1.1.3
	golang.org/x/mod v0.4.1 // indirect
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c
	golang.org/x/tools v0.1.0
)

replace crawshaw.io/sqlite v0.3.3-0.20201229170853-3aff1a1a78df => github.com/zombiezen/sqlite v0.3.3-0.20201229170853-3aff1a1a78df
