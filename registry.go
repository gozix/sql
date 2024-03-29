// Copyright 2018 Sergey Novichkov. All rights reserved.
// For the full copyright and license information, please view the LICENSE
// file that was distributed with this source code.

package sql

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/iqoption/nap"
)

// DEFAULT is default connection name.
const DEFAULT = "default"

type (
	// Config is registry configuration item.
	Config struct {
		Nodes           []string                      `json:"nodes"`
		Driver          string                        `json:"driver"`
		MaxOpenConns    int                           `json:"max_open_conns"`
		MaxIdleConns    int                           `json:"max_idle_conns"`
		ConnMaxLifetime time.Duration                 `json:"conn_max_lifetime"`
		AfterOpen       func(name string, db *nap.DB) `json:"-"`
	}

	// Configs are registry configurations.
	Configs map[string]Config

	// Registry is database connection registry.
	Registry struct {
		mux  sync.Mutex
		dbs  map[string]*nap.DB
		conf Configs
	}
)

var (
	// ErrUnknownConnection is error triggered when connection with provided name not founded.
	ErrUnknownConnection = errors.New("unknown connection")
)

// NewRegistry is registry constructor.
func NewRegistry(conf Configs) (*Registry, error) {
	return &Registry{
		dbs:  make(map[string]*nap.DB),
		conf: conf,
	}, nil
}

// Close is method for close connections.
func (r *Registry) Close() (err error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	for key, db := range r.dbs {
		if err = db.Close(); err != nil {
			return err
		}

		delete(r.dbs, key)
	}

	return nil
}

// Connection is default connection getter.
func (r *Registry) Connection() (*nap.DB, error) {
	return r.ConnectionWithName(DEFAULT)
}

// ConnectionWithName is connection getter by name.
func (r *Registry) ConnectionWithName(name string) (_ *nap.DB, err error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if db, ok := r.dbs[name]; ok {
		return db, nil
	}

	var db *nap.DB
	if db, err = r.open(name); err != nil {
		return nil, err
	}

	r.dbs[name] = db

	return r.dbs[name], nil
}

// Driver is default connection driver name getter.
func (r *Registry) Driver() (string, error) {
	return r.DriverWithName(DEFAULT)
}

// DriverWithName is driver name getter by name.
func (r *Registry) DriverWithName(name string) (string, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if value, ok := r.conf[name]; ok {
		return value.Driver, nil
	}

	return "", ErrUnknownConnection

}

func (r *Registry) open(name string) (db *nap.DB, err error) {
	var conf, ok = r.conf[name]
	if !ok {
		return nil, ErrUnknownConnection
	}
	if db, err = nap.Open(conf.Driver, strings.Join(conf.Nodes, ";")); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(conf.MaxOpenConns)
	db.SetMaxIdleConns(conf.MaxIdleConns)
	db.SetConnMaxLifetime(conf.ConnMaxLifetime)

	if err = db.Ping(); err != nil {
		return nil, err
	}

	if conf.AfterOpen != nil {
		conf.AfterOpen(name, db)
	}

	return db, nil
}
