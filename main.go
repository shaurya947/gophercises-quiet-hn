package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/shaurya947/gophercises-quiet-hn/hn"
)

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))

	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var client hn.Client
		ids, err := client.TopItems()
		if err != nil {
			http.Error(w, "Failed to load top stories", http.StatusInternalServerError)
			return
		}

		var stories []item
		itemChan := make(chan *item)
		storyIdToIdx := make(map[int]int)
		idx := 0
		for len(stories) < numStories && idx < len(ids) {
			goroutinesToLaunch := numStories - len(stories)
			indicesLeft := len(ids) - idx
			if indicesLeft < goroutinesToLaunch {
				goroutinesToLaunch = indicesLeft
			}

			for i := 0; i < goroutinesToLaunch; i++ {
				go fetchStoryItem(&client, ids[idx], itemChan)
				storyIdToIdx[ids[idx]] = idx
				idx++
			}

			for i := 0; i < goroutinesToLaunch; i++ {
				maybeItem := <-itemChan
				if maybeItem == nil {
					continue
				}

				item := *maybeItem
				if isStoryLink(item) {
					stories = append(stories, item)
				}
			}
		}

		sort.Slice(stories, func(i, j int) bool {
			return storyIdToIdx[stories[i].ID] < storyIdToIdx[stories[j].ID]
		})

		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err = tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

func fetchStoryItem(client *hn.Client, id int, itemChan chan *item) {
	hnItem, err := client.GetItem(id)
	if err != nil {
		itemChan <- nil
	}
	item := parseHNItem(hnItem)
	itemChan <- &item
}

func isStoryLink(item item) bool {
	return item.Type == "story" && item.URL != ""
}

func parseHNItem(hnItem hn.Item) item {
	ret := item{Item: hnItem}
	url, err := url.Parse(ret.URL)
	if err == nil {
		ret.Host = strings.TrimPrefix(url.Hostname(), "www.")
	}
	return ret
}

// item is the same as the hn.Item, but adds the Host field
type item struct {
	hn.Item
	Host string
}

type templateData struct {
	Stories []item
	Time    time.Duration
}
