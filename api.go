package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
)

const (
	YandexURL   = "https://translate.yandex.ru?source_lang=en&target_lang=ru"
	InputAreaID = "fakeArea"
)

const (
	defaultChromePath       = "driver/chrome-win64/chrome.exe"
	defaultChromeDriverPath = "driver/chromedriver.exe"
	defaultTranslateTimeout = 2 * time.Second
	defaultPort             = 14560
)

var ErrTranslateNotFound = errors.New("translate not found")

type TranslatorAPI struct {
	IsDebug          bool
	LogOut           io.Writer     // default is none
	Port             int           // default is 14560
	TranslateTimeout time.Duration // default is 2sec (and it is minimum value)
	ChromePath       string        // default is driver/chrome-win64/chrome.exe
	ChromeDriverPath string        // default is driver/chromedriver.exe

	service *selenium.Service
	wd      selenium.WebDriver

	inputArea  selenium.WebElement
	outputArea selenium.WebElement

	cleanups []func() error
}

func (t *TranslatorAPI) Init() (err error) {
	defer func() {
		if err != nil {
			t.Stop()
		}
	}()

	if t.LogOut == nil {
		t.LogOut = io.Discard
	}

	if t.TranslateTimeout < 2*time.Second {
		t.TranslateTimeout = defaultTranslateTimeout
	}

	if t.ChromePath == "" {
		t.ChromePath = defaultChromePath
	}

	if t.ChromeDriverPath == "" {
		t.ChromeDriverPath = defaultChromeDriverPath
	}

	if t.Port == 0 {
		t.Port = defaultPort
	}

	selenium.SetDebug(t.IsDebug)

	service, err := selenium.NewChromeDriverService(
		t.ChromeDriverPath, t.Port,
		selenium.Output(t.LogOut),
	)
	if err != nil {
		return fmt.Errorf("new chrome driver service: %w", err)
	}
	t.service = service
	t.cleanups = append(t.cleanups, func() error {
		if err := service.Stop(); err != nil {
			return fmt.Errorf("service stop: %w", err)
		}

		return nil
	})

	caps := selenium.Capabilities{"browserName": "chrome"}
	caps.AddChrome(chrome.Capabilities{
		Args: []string{"--headless"},
		Path: t.ChromePath,
	})

	wd, err := selenium.NewRemote(caps, fmt.Sprintf("http://localhost:%d/wd/hub", t.Port))
	if err != nil {
		return fmt.Errorf("new remote client: %w", err)
	}
	t.wd = wd
	t.cleanups = append(t.cleanups, func() error {
		if err := t.wd.Quit(); err != nil {
			return fmt.Errorf("webdriver quit: %w", err)
		}

		return nil
	})

	if err := wd.Get(YandexURL); err != nil {
		return fmt.Errorf("navigate browser to %s: %w", YandexURL, err)
	}

	time.Sleep(2 * time.Second)

	inputArea, err := wd.FindElement(selenium.ByID, "fakeArea")
	if err != nil {
		return fmt.Errorf("find input area: %w", err)
	}
	t.inputArea = inputArea

	outputArea, err := wd.FindElement(selenium.ByCSSSelector, "#translation")
	if err != nil {
		return fmt.Errorf("find output area: %w", err)
	}
	t.outputArea = outputArea

	return nil
}

func (t *TranslatorAPI) Stop() (err error) {
	for _, cleanup := range t.cleanups {
		err = errors.Join(err, cleanup())
	}

	t.cleanups = nil
	return
}

func (t *TranslatorAPI) Translate(text string) (string, error) {
	if text == "" {
		return "", errors.New("got empty text for translating, expected something")
	}

	if err := t.inputArea.Clear(); err != nil {
		return "", fmt.Errorf("clear input are: %w", err)
	}

	if err := t.inputArea.SendKeys(text); err != nil {
		return "", fmt.Errorf("send keys to input area: %w", err)
	}

	pollWait := 100 * time.Millisecond
	for i := 0; i < int(t.TranslateTimeout/pollWait); i++ {
		time.Sleep(pollWait)

		output, err := t.outputArea.Text()
		if err != nil {
			return "", fmt.Errorf("get text from output area: %w", err)
		}

		if output != "" {
			return output, nil
		}
	}

	return "", ErrTranslateNotFound
}
