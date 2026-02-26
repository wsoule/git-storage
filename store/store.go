package store

import "git.wyat.me/git-storage/object"

type ObjectStore interface {
	Put(obj *object.Object) (sha string, err error)
	Get(sha string) (*object.Object, error)
	Exists(sha string) (bool, error)
}
