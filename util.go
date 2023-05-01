package debanator

import (
	log "github.com/sirupsen/logrus"
)

func Md(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func Unwrap[T any](val T, err error) T {
	Md(err)
	return val
}
