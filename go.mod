module zombiezen.com/go/bass

go 1.15

require (
	crawshaw.io/sqlite v0.3.3-zombiezen
	github.com/google/go-cmp v0.3.1
	golang.org/x/sys v0.0.0-20190813064441-fde4db37ae7a
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
)

replace crawshaw.io/sqlite v0.3.3-zombiezen => github.com/zombiezen/sqlite v0.3.3-0.20201229170853-3aff1a1a78df
