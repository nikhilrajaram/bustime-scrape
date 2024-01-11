package main

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/gocolly/colly"
)

type Stop struct {
	StopId   int
	StopName string
}

func main() {
	c := colly.NewCollector()
	rootUrl := "https://bustime.mta.info/m/routes/"

	var routes []string
	var stops []Stop

	queryParamRe := regexp.MustCompile(".*\\?q=(.*)")
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Request.URL.String() != rootUrl {
			return
		}

		url := e.Attr("href")
		match := queryParamRe.FindStringSubmatch(url)
		if len(match) != 2 {
			return
		}
		route := match[1]
		routes = append(routes, route)

		e.Request.Ctx.Put("fromRoutesPage", "1")
		e.Request.Visit(url)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		if e.Request.Ctx.Get("fromRoutesPage") != "1" {
			return
		}

		url := e.Attr("href")
		match := queryParamRe.FindStringSubmatch(url)
		if len(match) != 2 {
			return
		}

		stopId, err := strconv.Atoi(match[1])
		if err != nil {
			return
		}
		stopName := e.Text

		stops = append(stops, Stop{StopId: stopId, StopName: stopName})
	})

	c.Visit(rootUrl)
	c.Wait()

	fmt.Println(routes)
	fmt.Println(stops)
}
