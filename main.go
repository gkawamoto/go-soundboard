package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/gkawamoto/go-soundboard/audio"
	"github.com/gkawamoto/go-soundboard/ui"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

type data struct {
	Sounds map[string]string `json:"sounds"`
}

func main() {
	workingDir := ""
	flag.StringVar(&workingDir, "path", ".", "Path where the soundboard is")
	flag.Parse()

	err := os.Chdir(workingDir)
	if err != nil {
		log.Fatal(err)
	}

	data, err := loadData()
	if err != nil {
		log.Fatal(err)
	}

	p := &controller{}
	app := ui.NewApp(p)
	p.view = app
	p.data = data

	updateSoundList(app, data)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

type controller struct {
	view *ui.App
	data *data
}

func (c *controller) PlaySound(file string) error {
	return audio.Play(file)
}

func (c *controller) DownloadFile(file string) (string, error) {
	err := os.MkdirAll("download", 0700)
	if err != nil {
		return "", err
	}

	url, err := url.Parse(file)
	if err != nil {
		return "", err
	}

	res, err := http.Get(url.String())
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	result := filepath.Join("download", filepath.Base(file))

	err = os.WriteFile(result, data, 0600)
	if err != nil {
		return "", err
	}
	return result, nil
}

func (c *controller) Save() error {
	data, err := json.Marshal(c.data)
	if err != nil {
		return err
	}

	err = os.WriteFile("data.json", data, 0600)
	if err != nil {
		return err
	}

	return nil
}

func (c *controller) Load() error {
	data, err := loadData()
	if err != nil {
		return err
	}

	c.data = data
	updateSoundList(c.view, c.data)
	return nil
}

func (c *controller) AddFile(file string, key rune) {
	c.data.Sounds[string(key)] = file
	updateSoundList(c.view, c.data)
}

func (c *controller) FileSearch(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	result := []string{}
	err := filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".mp3" {
			return nil
		}

		if fuzzy.Match(text, path) {
			result = append(result, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *controller) SearchAPI(text string) ([]string, error) {
	if text == "" {
		return nil, nil
	}

	if utf8.RuneCountInString(text) < 3 {
		return nil, nil
	}

	v := url.Values{}
	v.Add("format", "json")
	v.Add("name", text)

	res, err := http.Get("https://www.myinstants.com/api/v1/instants/?" + v.Encode())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("api returned %d", res.StatusCode)
	}

	content, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	data := struct {
		Results []struct {
			Sound string `json:"sound"`
		} `json:"Results"`
	}{}

	err = json.Unmarshal(content, &data)
	if err != nil {
		return nil, err
	}
	result := []string{}

	for _, s := range data.Results {
		result = append(result, s.Sound)
	}

	return result, nil
}

func (c *controller) DeleteItem(index int, key rune) {
	if len(c.data.Sounds) == 0 {
		return
	}

	delete(c.data.Sounds, string(key))
	updateSoundList(c.view, c.data)
	c.view.SetSelectedSound(index)
}

func (c *controller) StopAllSounds() {
	audio.StopAll()
}

func updateSoundList(app *ui.App, data *data) {
	app.ClearSoundList()
	for _, k := range ui.ValidSoundKeys {
		file, ok := data.Sounds[string(k)]
		if !ok {
			continue
		}
		app.AddSound(file, k)
	}
}

func loadData() (*data, error) {
	content, err := os.ReadFile("data.json")
	if err != nil {
		return &data{map[string]string{}}, nil
	}

	result := &data{}
	err = json.Unmarshal(content, result)
	if err != nil {
		return nil, err
	}

	return result, err
}
