package audio

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var nowPlaying = &sync.Map{}

func Play(filepath string) error {
	if strings.HasPrefix(filepath, "http://") || strings.HasPrefix(filepath, "https://") {
		return playRemote(filepath)
	}
	return playLocal(filepath)
}

func playRemote(filepath string) error {
	url, err := url.Parse(filepath)
	if err != nil {
		return err
	}

	res, err := http.Get(url.String())
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	cmd := exec.Command("mpg321", "-")
	nowPlaying.Store(cmd, true)
	defer nowPlaying.Delete(cmd)
	cmd.Stdin = bytes.NewBuffer(data)
	return cmd.Run()
}

func playLocal(filepath string) error {
	cmd := exec.Command("mpg321", filepath)
	nowPlaying.Store(cmd, true)
	defer nowPlaying.Delete(cmd)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func StopAll() {
	nowPlaying.Range(func(key, value interface{}) bool {
		cmd := key.(*exec.Cmd)
		_ = cmd.Process.Signal(os.Interrupt)
		return true
	})
}
