// Copyright 2010 The Walk Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package walk

import . "github.com/lxn/go-winapi"

type Settings interface {
	Get(key string) (string, bool)
	Put(key, value string) error
	Load() error
	Save() error
}

type Persistable interface {
	Persistent() bool
	SetPersistent(value bool)
	SaveState() error
	RestoreState() error
}

type Application struct {
	organizationName   string
	productName        string
	settings           Settings
	exiting            bool
	exitCode           int
	panickingPublisher ErrorEventPublisher
}

var appSingleton *Application = &Application{}

func App() *Application {
	return appSingleton
}

func (app *Application) OrganizationName() string {
	return app.organizationName
}

func (app *Application) SetOrganizationName(value string) {
	app.organizationName = value
}

func (app *Application) ProductName() string {
	return app.productName
}

func (app *Application) SetProductName(value string) {
	app.productName = value
}

func (app *Application) Settings() Settings {
	return app.settings
}

func (app *Application) SetSettings(value Settings) {
	app.settings = value
}

func (app *Application) Exit(exitCode int) {
	app.exiting = true
	app.exitCode = exitCode
	PostQuitMessage(int32(exitCode))
}

func (app *Application) ExitCode() int {
	return app.exitCode
}

func (app *Application) Panicking() *ErrorEvent {
	return app.panickingPublisher.Event()
}
