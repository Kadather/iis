package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coyove/iis/cmd/ch/id"
	"github.com/etcd-io/bbolt"
)

var (
	errNoBucket = errors.New("")
	bkPost      = []byte("post")
)

type Manager struct {
	db *bbolt.DB
	mu sync.Mutex
}

func NewManager(path string) (*Manager, error) {
	db, err := bbolt.Open(path, 0700, &bbolt.Options{
		FreelistType: bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, err
	}

	m := &Manager{db: db}

	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bkPost)
		return err
	}); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) FindPosts(tag string, cursor []byte, n int) (a []*Article, prev []byte, next []byte, err error) {
	err = m.db.View(func(tx *bbolt.Tx) error {
		bk := tx.Bucket(bkPost)

		var res [][2][]byte
		res, next = scanBucketDesc(bk, tag, cursor, n)
		prev = substractCursorN(bk, tag, cursor, n)

		if tag == "" {
			a = m.mget(tx, true, res)
			for i := len(a) - 1; i >= 0; i-- {
				if a[i].Parent != nil {
					a = append(a[:i], a[i+1:]...)
				}
			}

		} else {
			a = m.mget(tx, false, res)
		}

		if id.ParseID(next).Tag() != tag {
			next = nil
		}

		return nil
	})
	return
}

func (m *Manager) insertTag(bk *bbolt.Bucket, b string, k, v []byte) error {
	kid := id.ParseID(k)
	kid.SetTag(b)
	kid.SetHeader(id.HeaderAuthorTag)
	return bk.Put(kid.Marshal(), v)
}

func (m *Manager) deleteTags(bk *bbolt.Bucket, k []byte, tags ...string) (err error) {
	kid := id.ParseID(k)
	kid.SetHeader(id.HeaderAuthorTag)
	for _, tag := range tags {
		kid.SetTag(tag)
		err = bk.Delete(kid.Marshal())
	}
	return
}

func (m *Manager) PostPost(a *Article) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		bk := tx.Bucket(bkPost)
		a.Index = int64(bk.Stats().KeyN + 1)

		if err := bk.Put(a.ID, a.marshal()); err != nil {
			return err
		}
		if err := m.insertTag(bk, a.Author, a.ID, a.ID); err != nil {
			return err
		}
		if err := m.insertTag(bk, "#"+a.Category, a.ID, a.ID); err != nil {
			return err
		}
		return nil
	})
}

func (m *Manager) PostReply(parent []byte, a *Article) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, err := m.GetArticle(parent)
	if err != nil {
		return err
	}
	if p.Locked {
		return fmt.Errorf("locked parent")
	}
	if p.Replies >= 16000 {
		return fmt.Errorf("too many replies")
	}

	pid := id.ParseID(parent)

	p.ReplyTime = uint32(time.Now().Unix())
	p.Replies++

	a.Parent = parent
	a.Category = ""
	a.Title = "RE: " + p.Title
	a.Index = p.Replies

	if !pid.RIndexAppend(int16(p.Replies)) {
		return fmt.Errorf("too deep")
	}

	pid.SetHeader(id.HeaderReply)
	a.ID = pid.Marshal()

	return m.db.Update(func(tx *bbolt.Tx) error {
		main := tx.Bucket(bkPost)
		if err := main.Put(a.ID, a.marshal()); err != nil {
			return err
		}
		if err := main.Put(p.ID, p.marshal()); err != nil {
			return err
		}
		if err := m.insertTag(main, a.Author, a.ID, a.ID); err != nil {
			return err
		}
		if p.Author != a.Author {
			return m.insertTag(main, p.Author, id.NewID(id.HeaderAuthorTag, "").Marshal(), a.ID)
		}
		return nil
	})
}

func (m *Manager) GetArticle(id []byte) (*Article, error) {
	a := &Article{}
	return a, m.db.View(func(tx *bbolt.Tx) error {
		return a.unmarshal(tx.Bucket(bkPost).Get(id))
	})
}

func (m *Manager) UpdateArticle(a *Article, oldcat string) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		main := tx.Bucket(bkPost)

		if a.Category != oldcat {
			if err := m.deleteTags(main, a.ID, "#"+oldcat); err != nil {
				return err
			}
			if err := m.insertTag(main, "#"+a.Category, a.ID, a.ID); err != nil {
				return err
			}
		}

		return main.Put(a.ID, a.marshal())
	})
}

func (m *Manager) DeleteArticle(a *Article) error {
	return m.db.Update(func(tx *bbolt.Tx) error {
		main := tx.Bucket(bkPost)
		if err := main.Delete(a.ID); err != nil {
			return err
		}
		return m.deleteTags(main, a.ID, "#"+a.Category, a.Author)
	})
}
