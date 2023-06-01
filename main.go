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
	"sync"
	"time"

	"github.com/shaurya947/gophercises-quiet-hn/hn"
)

var cache topStoriesCache

func main() {
	// parse flags
	var port, numStories int
	flag.IntVar(&port, "port", 3000, "the port to start the web server on")
	flag.IntVar(&numStories, "num_stories", 30, "the number of top stories to display")
	flag.Parse()

	tpl := template.Must(template.ParseFiles("./index.gohtml"))

	http.HandleFunc("/", handler(numStories, tpl))

	go updateTopStoriesBackground(numStories)
	// Start the server
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func handler(numStories int, tpl *template.Template) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var stories []item
		for {
			cache.RWMutex.RLock()
			initialized := cache.initialized
			err := cache.error
			stories = make([]item, len(cache.topStories))
			copy(stories, cache.topStories)
			cache.RWMutex.RUnlock()

			if !initialized {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if err != nil && len(stories) == 0 {
				http.Error(w, "Failed to load top stories", http.StatusInternalServerError)
				return
			}

			break
		}

		data := templateData{
			Stories: stories,
			Time:    time.Now().Sub(start),
		}
		err := tpl.Execute(w, data)
		if err != nil {
			http.Error(w, "Failed to process the template", http.StatusInternalServerError)
			return
		}
	})
}

func updateTopStoriesBackground(numStories int) {
	var client hn.Client
	for {
		ids, err := client.TopItems()
		if err != nil {
			cache.RWMutex.Lock()
			cache.error = err
			cache.initialized = true
			cache.RWMutex.Unlock()
			continue
		}

		var stories []item
		quitSignal := make(chan struct{})
		storyChan := make(chan item)
		wg := waitGroupWithCount{}
		i := 0
		storyIdToIdx := make(map[int]int)
	loop:
		for {
			select {
			case story := <-storyChan:
				stories = append(stories, story)
				if len(stories) >= numStories {
					close(quitSignal)
					break loop
				}
			default:
				goroutineCount := wg.GetCount()
				if i < len(ids) && goroutineCount < 10 {
					wg.Add(1)
					go fetchStoryItem(&wg, &client, ids[i], storyChan, quitSignal)
					storyIdToIdx[ids[i]] = i
					i++
				} else if i >= len(ids) && goroutineCount == 0 {
					close(quitSignal)
					break loop
				}
			}
		}

		sort.Slice(stories, func(i, j int) bool {
			return storyIdToIdx[stories[i].ID] < storyIdToIdx[stories[j].ID]
		})

		cache.RWMutex.Lock()
		cache.initialized = true
		cache.error = nil
		cache.topStories = make([]item, len(stories))
		copy(cache.topStories, stories)
		cache.RWMutex.Unlock()

		time.Sleep(10 * time.Second)
	}
}

func fetchStoryItem(wg *waitGroupWithCount, client *hn.Client, id int, storyChan chan item, quitSignal chan struct{}) {
	defer wg.Done()

	hnItem, err := client.GetItem(id)
	if err != nil {
		return
	}

	item := parseHNItem(hnItem)
	if !isStoryLink(item) {
		return
	}

	select {
	case storyChan <- item:
	case <-quitSignal:
	}
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

type topStoriesCache struct {
	topStories  []item
	initialized bool
	sync.RWMutex
	error
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
