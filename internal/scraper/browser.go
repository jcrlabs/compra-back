package scraper

import (
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

var (
	globalBrowser *rod.Browser
	browserOnce   sync.Once
)

func getBrowser() *rod.Browser {
	browserOnce.Do(func() {
		u := launcher.New().
			Headless(true).
			NoSandbox(true).
			Set("disable-gpu", "").
			Set("disable-dev-shm-usage", "").
			Bin("/usr/bin/chromium").
			MustLaunch()
		globalBrowser = rod.New().ControlURL(u).MustConnect()
	})
	return globalBrowser
}
