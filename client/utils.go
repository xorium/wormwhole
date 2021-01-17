package client

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

const (
	lockFilePath       = "/tmp/.w_hole.lock"
	touchingLockPeriod = 5 * time.Second
)

func (c *Client) goToBinaryDir() {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Println(err)
		return
	}
	if err = os.Chdir(dir); err != nil {
		log.Println(err)
	}
}

func (c *Client) saveSettings() {
	c.goToBinaryDir()
	f, err := os.Create(settingsFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() { _ = f.Close() }()

	c.RLock()
	content, err := json.Marshal(c.settings)
	c.RUnlock()
	if err != nil {
		log.Println(err)
		return
	}
	if _, err := f.Write(content); err != nil {
		log.Println(err)
	}
}

func (c *Client) loadSettings() {
	c.goToBinaryDir()
	f, err := os.Create(settingsFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() { _ = f.Close() }()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println(err)
		return
	}
	if len(content) == 0 {
		return
	}

	c.Lock()
	defer c.Unlock()
	if err := json.Unmarshal(content, &c.settings); err != nil {
		log.Println(err)
		return
	}
}

func (c *Client) getSettingString(key string) string {
	c.RLock()
	value, ok := c.settings[key]
	c.RUnlock()
	if !ok {
		return ""
	}
	strValue, ok := value.(string)
	if !ok {
		return ""
	}
	return strValue
}

func (c *Client) handleShutdownSignals() {
	go func() {
		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, os.Interrupt, os.Kill)
		<-sigChan
		if err := os.Remove(lockFilePath); err != nil {
			log.Println("error while removing the lock file: ", err)
		}
		if c.conn != nil {
			_ = c.conn.Close()
		}
		os.Exit(0)
	}()
}

func (c *Client) startLockMaintaining() {
	f, err := os.Create(lockFilePath)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	_ = f.Close()

	c.handleShutdownSignals()
	// touching the lock file
	for {
		currentTime := time.Now().Local()
		err = os.Chtimes(lockFilePath, currentTime, currentTime)
		if err != nil {
			log.Println("error while touching the lock file: ", err)
			os.Exit(0)
		}
		time.Sleep(touchingLockPeriod)
	}
}

func (c *Client) checkLock() {
	stat, err := os.Stat(lockFilePath)
	if err != nil && os.IsNotExist(err) {
		go c.startLockMaintaining()
		return
	}
	if time.Now().Local().Sub(stat.ModTime()) > touchingLockPeriod*2 {
		go c.startLockMaintaining()
		return
	}
	log.Println("Another bot has always been started.")
	os.Exit(0)
}
