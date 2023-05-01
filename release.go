package debanator

import (
	"pault.ag/go/debian/control"
	"pault.ag/go/debian/dependency"
)

type Release struct {
	Suite         string
	Architectures []dependency.Arch
	Components    string
	Date          string
	SHA1           []control.SHA1FileHash `delim:"\n" strip:"\n\r\t "`
	SHA256        []control.SHA256FileHash`delim:"\n" strip:"\n\r\t "`
	MD5Sum        []control.MD5FileHash`delim:"\n" strip:"\n\r\t "`
}
