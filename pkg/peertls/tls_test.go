// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
)

var quickConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		randHex := fmt.Sprintf("%x", r.Uint32())
		values[0] = reflect.ValueOf(randHex)
	},
}

var quickTLSOptionsConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		for i := range [3]bool{} {
			randHex := fmt.Sprintf("%x", r.Uint32())
			values[i] = reflect.ValueOf(randHex)
		}

		for i := range [2]bool{} {
			randBool := r.Uint32()&0x01 != 0
			values[i+3] = reflect.ValueOf(randBool)
		}
	},
}

var quickLog = func(msg string, obj interface{}, err error) {
	if msg != "" {
		fmt.Printf("%s:\n", msg)
	}

	if obj != nil {
		fmt.Printf("obj: %v\n", obj)
	}

	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}

type tlsFileOptionsTestCase struct {
	tlsFileOptions *TLSFileOptions
	before         func(*tlsFileOptionsTestCase) error
	after          func(*tlsFileOptionsTestCase) error
}

func TestNewTLSFileOptions(t *testing.T) {
	f := func(cert, key, hosts string, client, overwrite bool) (_ bool) {
		tempPath, err := ioutil.TempDir("", "TestNewTLSFileOptions")
		assert.NoError(t, err)
		defer os.RemoveAll(tempPath)

		var subExt string
		if client {
			subExt = "client"
		} else {
			subExt = "leaf"
		}

		certBasePath := filepath.Join(tempPath, cert)
		keyBasePath := filepath.Join(tempPath, key)
		certPath := fmt.Sprintf("%s.%s.cert", certBasePath, subExt)
		keyPath := fmt.Sprintf("%s.%s.key", keyBasePath, subExt)
		opts, err := NewTLSFileOptions(certBasePath, keyBasePath, hosts, client, true, overwrite)
		if !assert.NoError(t, err) {
			quickLog("", nil, err)
			return false
		}

		if !assert.Equal(t, opts.RootCertRelPath, fmt.Sprintf("%s.%s.cert", certBasePath, "root")) {
			return false
		}
		if !assert.Equal(t, opts.RootKeyRelPath, fmt.Sprintf("%s.%s.key", keyBasePath, "root")) {
			return false
		}

		if client {
			if !assert.NotEmpty(t, opts.ClientCertificate) {
				return false
			}
			if !assert.NotEmpty(t, opts.ClientCertificate.PrivateKey) {
				return false
			}
			if !assert.Equal(t, opts.ClientCertRelPath, certPath) {
				return false
			}
			if !assert.Equal(t, opts.ClientKeyRelPath, keyPath) {
				return false
			}
		} else {
			if !assert.NotEmpty(t, opts.LeafCertificate) {
				return false
			}
			if !assert.NotEmpty(t, opts.LeafCertificate.PrivateKey) {
				return false
			}
			if !assert.Equal(t, opts.LeafCertRelPath, certPath) {
				return false
			}
			if !assert.Equal(t, opts.LeafKeyRelPath, keyPath) {
				return false
			}
		}

		if !assert.Equal(t, opts.Hosts, hosts) {
			return false
		}
		if !assert.Equal(t, opts.Client, client) {
			return false
		}
		if !assert.Equal(t, opts.Overwrite, overwrite) {
			return false
		}

		return true
	}

	err := quick.Check(f, quickTLSOptionsConfig)
	assert.NoError(t, err)
}

func TestEnsureAbsPath(t *testing.T) {
	f := func(val string) (_ bool) {
		opts := &TLSFileOptions{
			RootCertRelPath:   fmt.Sprintf("%s.root.cert", val),
			RootKeyRelPath:    fmt.Sprintf("%s.root.key", val),
			LeafCertRelPath:   fmt.Sprintf("%s.leaf.cert", val),
			LeafKeyRelPath:    fmt.Sprintf("%s.leaf.key", val),
			ClientCertRelPath: fmt.Sprintf("%s.client.cert", val),
			ClientKeyRelPath:  fmt.Sprintf("%s.client.key", val),
		}

		opts.EnsureAbsPaths()

		// TODO(bryanchriswhite) cleanup/refactor
		for _, requiredRole := range opts.requiredFiles() {
			for absPtr, role := range opts.pathRoleMap() {
				if role == requiredRole {
					if *absPtr == "" {
						msg := fmt.Sprintf("absolute path for %s is empty string", fileLabels[role])
						quickLog(msg, opts, nil)
						return false
					}
				}
			}
		}

		for _, requiredRole := range opts.requiredFiles() {
			for absPtr, role := range opts.pathRoleMap() {
				base := filepath.Base
				if role == requiredRole {
					relPath := opts.pathMap()[absPtr]
					if base(*absPtr) != base(relPath) {
						quickLog("basenames don't match", opts, nil)
						return false
					}
				}
			}
		}

		return true
	}

	err := quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestGenerate(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestGenerate")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)

		opts := &TLSFileOptions{
			RootCertAbsPath: RootCertPath,
			RootKeyAbsPath:  RootKeyPath,
			LeafCertAbsPath: LeafCertPath,
			LeafKeyAbsPath:  LeafKeyPath,
			Create:          true,
			Overwrite:       false,
			Hosts:           "127.0.0.1",
		}

		if err := opts.generateTLS(); err != nil {
			quickLog("generateTLS error", opts, err)
			return false
		}

		rootCert, err := LoadCert(RootCertPath, RootKeyPath)
		if err != nil {
			quickLog("error root loading cert", opts, err)
			return false
		}

		leafCert, err := LoadCert(LeafCertPath, LeafKeyPath)
		if err != nil {
			quickLog("error leaf loading cert", opts, err)
			return false
		}

		privKeyBytes := func(key crypto.PrivateKey) []byte {
			switch key.(type) {
			case *ecdsa.PrivateKey:
			default:
				quickLog("non-ecdsa private key", key, nil)
				panic("non-ecdsa private key")
			}
			ecKey := key.(*ecdsa.PrivateKey)
			b, err := x509.MarshalECPrivateKey(ecKey)
			assert.NoError(t, err)

			return b
		}

		certsMatch := func(c1, c2 *tls.Certificate) bool {
			for i, cert := range c1.Certificate {
				if bytes.Compare(cert, c2.Certificate[i]) != 0 {
					quickLog("certs don't match", opts, err)
					return false
				}
			}

			return true
		}

		keysMatch := func(k1, k2 []byte) bool {
			if bytes.Compare(k1, k2) != 0 {
				quickLog("keys don't match", opts, err)
				return false
			}

			return true
		}

		if !certsMatch(leafCert, opts.LeafCertificate) {
			quickLog("certs don't match", opts, nil)
			return false
		}

		if !keysMatch(
			privKeyBytes(leafCert.PrivateKey),
			privKeyBytes(rootCert.PrivateKey),
			) {
			quickLog("keys don't match", opts, nil)
			return false
		}

		return true
	}

	err = quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func  TestLoadCert(t *testing.T) {
	t.SkipNow()
}

func TestEnsureExists_Create(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Create")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)

		opts := &TLSFileOptions{
			RootCertAbsPath: RootCertPath,
			RootKeyAbsPath:  RootKeyPath,
			LeafCertAbsPath: LeafCertPath,
			LeafKeyAbsPath:  LeafKeyPath,
			Create:          true,
			Overwrite:       false,
			Hosts:           "127.0.0.1",
		}

		err := opts.EnsureExists()
		if err != nil {
			quickLog("ensureExists err", opts, err)
			return false
		}

		for _, requiredRole := range opts.requiredFiles() {
			for absPtr, role := range opts.pathRoleMap() {
				if role == requiredRole {
					if _, err = os.Stat(*absPtr); err != nil {
						quickLog("path doesn't exist", opts, nil)
						return false
					}
				}
			}
		}

		// TODO: check for *tls.Certificate and pubkey

		return true
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestEnsureExists_Overwrite(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Overwrite")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)

		checkFiles := func(opts *TLSFileOptions, checkSize bool) bool {
			for _, requiredRole := range opts.requiredFiles() {
				for absPtr, role := range opts.pathRoleMap() {
					if role == requiredRole {
						f, err := os.Stat(*absPtr)

						if err != nil {
							quickLog(fmt.Sprintf("%s path doesn't exist", *absPtr), opts, nil)
							return false
						}

						if checkSize && !(f.Size() > 0) {
							quickLog(fmt.Sprintf("%s has size 0", *absPtr), opts, nil)
							return false
						}
					}
				}
			}

			return true
		}

		requiredFiles := []string{
			RootCertPath,
			RootKeyPath,
			LeafCertPath,
			LeafKeyPath,
		}

		for _, path := range requiredFiles {
			if c, err := os.Create(path); err != nil {
				quickLog("", nil, errs.Wrap(err))
				return false
			} else {
				c.Close()
			}
		}

		opts := &TLSFileOptions{
			RootCertAbsPath: RootCertPath,
			RootKeyAbsPath:  RootKeyPath,
			LeafCertAbsPath: LeafCertPath,
			LeafKeyAbsPath:  LeafKeyPath,
			Create:          true,
			Overwrite:       true,
			Hosts:           "127.0.0.1",
		}

		// Ensure files exist to be overwritten
		checkFiles(opts, false)

		if err := opts.EnsureExists(); err != nil {
			quickLog("ensureExists err", opts, err)
			return false
		}

		checkFiles(opts, true)

		return true
	}

	err = quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestEnsureExists_NotExistError(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_NotExistError")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)

		opts := &TLSFileOptions{
			RootCertAbsPath: RootCertPath,
			RootKeyAbsPath:  RootKeyPath,
			LeafCertAbsPath: LeafCertPath,
			LeafKeyAbsPath:  LeafKeyPath,
			Create:          false,
			Overwrite:       false,
			Hosts:           "127.0.0.1",
		}

		if err := opts.EnsureExists(); err != nil {
			if IsNotExist(err) {
				return true
			}

			quickLog("unexpected err", opts, err)
			return false
		}

		quickLog("didn't error but should've", opts, nil)
		return false
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestNewTLSConfig(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestNewPeerTLS")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	basePath := filepath.Join(tempPath, "TestNewPeerTLS")

	opts, err := NewTLSFileOptions(
		basePath,
		basePath,
		"127.0.0.1",
		false,
		true,
		false,
	)
	assert.NoError(t, err)

	config := opts.NewTLSConfig(nil)
	assert.Equal(t, *opts.LeafCertificate, config.Certificates[0])
}
