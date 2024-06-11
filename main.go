package main

import (
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"path/filepath"

	"encoding/csv"

	"github.com/gocolly/colly"
)

type RWSlice[T any] struct {
	s  []T
	mu sync.RWMutex
}

func (rws *RWSlice[T]) append(val T) {
	rws.mu.Lock()
	defer rws.mu.Unlock()
	rws.s = append(rws.s, val)
}

type RWMap[K comparable, V any] struct {
	m  map[K]V
	mu sync.RWMutex
}

func (rwm *RWMap[K, V]) add(key K, val V) {
	rwm.mu.Lock()
	defer rwm.mu.Unlock()
	rwm.m[key] = val
}

func (rwm *RWMap[K, V]) get(key K) (V, bool) {
	rwm.mu.RLock()
	defer rwm.mu.RUnlock()
	val, ok := rwm.m[key]
	return val, ok
}

func (rwm *RWMap[K, V]) has(key K) bool {
	rwm.mu.RLock()
	defer rwm.mu.RUnlock()
	_, ok := rwm.m[key]
	return ok
}

// route pages have the routeId as the first query param
// route IDs are strings with alphanumeric(s) followed by digit(s)
var routeUrlRe = regexp.MustCompile(".*?q=[a-zA-Z]+[0-9]+")

// returns true if URL corresponds to a bus route, otherwise empty string
func isRoutePage(url string) bool {
	return routeUrlRe.FindString(url) != ""
}

// a little hacky. grabs the first query param of the URL which has included
// stop/route IDs in tests thus far
var firstQueryParamRe = regexp.MustCompile(".*\\?q=([^&]*)[&]?.*")

// returns first query param for the url if provided, otherwise empty string
func getFirstQueryParam(url string) string {
	match := firstQueryParamRe.FindStringSubmatch(url)
	if len(match) != 2 {
		return ""
	}

	return match[1]
}

func toCSV(path string, headers []string, data [][]string) {
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Fatalf("error creating directory %s: %s\n", dir, err)
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatalf("error opening file %s: %s\n", path, err)
	}
	w := csv.NewWriter(f)
	if err := w.Write(headers); err != nil {
		log.Fatalf("error writing headers to %s: %s\n", path, err)
	}
	for _, datum := range data {
		if err := w.Write(datum); err != nil {
			log.Fatalf("error writing data to %s: %s\n", path, err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		log.Fatalf("error flushing writer buffer: %s\n", err)
	}
}

func main() {
	// root URL, all stops/routes can be crawled from here
	allRoutesUrl := "https://bustime.mta.info/m/routes/"

	// route ID set
	routeMap := RWMap[string, bool]{m: make(map[string]bool)}
	// stop ID => routes map
	routesForStopMap := RWMap[string, []string]{m: make(map[string][]string)}
	// stop ID => stop name map
	stopMap := RWMap[string, string]{m: make(map[string]string)}
	// for any unsuccessful requests
	errors := RWSlice[[]string]{}
	// visited URL set
	visited := RWMap[string, bool]{m: make(map[string]bool)}

	c := colly.NewCollector(
		// crawling goes root page (links to all routes) => individual route page
		//   => stop page so we only need to crawl 2 deep
		colly.MaxDepth(2),
		colly.Async(true),
	)

	// all routes page handler
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		currentUrl := e.Request.URL.String()
		defer visited.add(currentUrl, true)
		if currentUrl != allRoutesUrl {
			return
		}

		url := e.Attr("href")
		if visited.has(url) {
			return
		}
		routeId := getFirstQueryParam(url)
		if routeId == "" {
			return
		}
		routeMap.add(routeId, true)

		e.Request.Visit(url)
	})

	// individual route page handler
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		currentUrl := e.Request.URL.String()
		defer visited.add(currentUrl, true)
		if !isRoutePage(currentUrl) {
			return
		}

		url := e.Attr("href")
		if isRoutePage(url) {
			return
		}
		stopId := getFirstQueryParam(url)
		if stopId == "" {
			return
		}
		stopName := e.Text

		stopMap.add(stopId, stopName)

		stopsForRoutes, ok := routesForStopMap.get(stopId)
		if ok {
			routesForStopMap.add(stopId, append(stopsForRoutes, stopId))
		} else {
			routesForStopMap.add(stopId, []string{stopId})
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		errors.append([]string{strconv.Itoa(r.StatusCode), err.Error()})
	})

	c.Visit(allRoutesUrl)
	c.Wait()

	var routeData [][]string
	for routeId := range routeMap.m {
		routeData = append(routeData, []string{routeId})
	}
	toCSV("out/routes.csv", []string{"routeId"}, routeData)
	var stopData [][]string
	for stopId, stopName := range stopMap.m {
		routes, _ := routesForStopMap.get(stopId)
		stopData = append(stopData, []string{stopId, stopName, strings.Join(routes, ",")})
	}
	toCSV("out/stops.csv", []string{"stopId", "stopName", "routes"}, stopData)
	if len(errors.s) > 1 {
		errorFile := "out/errors.csv"
		toCSV(errorFile, []string{"status code", "error message"}, errors.s)
		log.Fatalf("encountered errors when scraping. see %s\n", errorFile)
	}
}
