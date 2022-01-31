package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	//"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/gocolly/colly/v2"
	_ "github.com/heroku/x/hmetrics/onload"
)

type EntsoeRequest struct {

	// defining struct variables
	Name      string
	Capital   string
	Continent string
}

func getDateFromString(datetime string) time.Time {
	year, _ := strconv.Atoi(datetime[:4])
	tmpmonth, _ := strconv.Atoi(datetime[5:7])
	month := time.Month(tmpmonth)
	day, _ := strconv.Atoi(datetime[8:10])
	hour, _ := strconv.Atoi(datetime[11:13])
	minute, _ := strconv.Atoi(datetime[14:16])
	sec, _ := strconv.Atoi(datetime[17:19])
	newtmp := time.Date(year, month, day, hour, minute, sec, 0, time.Now().Location())
	return newtmp
}

/*
requestDate: in the form of 20.12.2022
*/
func getCountryTransmissions(requestDate string, country_pair string, timestamp string) string {
	fmt.Println(requestDate, country_pair, timestamp)
	datetimestring := requestDate + "+00%3A00%7CCET%7CDAY"
	// country_pair := "CTY|10YAT-APG------L!CTY_CTY|10YAT-APG------L_CTY_CTY|10YCZ-CEPS-----N"
	// timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	url := "https://transparency.entsoe.eu/transmission-domain/physicalFlow/show?name=&defaultValue=false&viewType=TABLE&areaType=BORDER_CTY&atch=false&dateTime.dateTime=" + datetimestring + "&border.values=" + country_pair + "&dateTime.timezone=CET_CEST&dateTime.timezone_input=CET+(UTC%2B1)+%2F+CEST+(UTC%2B2)&_=" + timestamp
	// https://transparency.entsoe.eu/transmission-domain/physicalFlow/show?name=&defaultValue=false&viewType=TABLE&areaType=BORDER_CTY&atch=false&dateTime.dateTime=300.130.30007+00%3A00%7CCET%7CDAY&border.values=CTY|10YAT-APG------L!CTY_CTY|10YAT-APG------L_CTY_CTY|10YCZ-CEPS-----N&dateTime.timezone=CET_CEST&dateTime.timezone_input=CET+(UTC%2B1)+%2F+CEST+(UTC%2B2)&_=1643542115
	// https://transparency.entsoe.eu/transmission-domain/physicalFlow/show?name=&defaultValue=false&viewType=TABLE&areaType=BORDER_CTY&atch=false&dateTime.dateTime=09.01.2022+00%3A00%7CCET%7CDAY&border.values=CTY|10YAT-APG------L!CTY_CTY|10YAT-APG------L_CTY_CTY|10YCZ-CEPS-----N&dateTime.timezone=CET_CEST&dateTime.timezone_input=CET+(UTC%2B1)+%2F+CEST+(UTC%2B2)&_=1643535605.0278914
	// https://transparency.entsoe.eu/transmission-domain/physicalFlow/show?name=&defaultValue=false&viewType=TABLE&areaType=BORDER_CTY&atch=false&dateTime.dateTime=20.12.202200%3A00%7CCET%7CDAY&border.values=CTY|10YAT-APG------L!CTY_CTY|10YAT-APG------L_CTY_CTY|10YCZ-CEPS-----N&dateTime.timezone=CET_CEST&dateTime.timezone_input=CET+(UTC%2B1)+%2F+CEST+(UTC%2B2)&_=1643542528
	return url

}

func ReadJsonEntsoe() map[string][]string {
	content, err := ioutil.ReadFile("./static/entsoe-transmissions.json")
	if err != nil {
		log.Fatal("Error when opening file: ", err)
	}
	var payload map[string][]string
	err = json.Unmarshal(content, &payload)
	if err != nil {
		log.Fatal("Error during Unmarshal(): ", err)
	}
	return payload
}

func getUrlContent(urlToGet string, ch chan<- string) {
	var (
		err     error
		content []byte
		resp    *http.Response
	)

	// GET content of URL
	if resp, err = http.Get(urlToGet); err != nil {
		return
	}
	defer resp.Body.Close()

	// Check if request was successful
	if resp.StatusCode != 200 {
		return
	}

	// Read the body of the HTTP response
	if content, err = ioutil.ReadAll(resp.Body); err != nil {
		return
	}

	ch <- string(content)
	return
}

type lineData struct {
	border              string   `json:border`
	timeframe           string   `json:timeframe`
	upstreamDirection   []string `json:upstreamDirection`
	downstreamDirection []string `json:downstreamDirection`
	upstream            string   `json:upstream`
	downstream          string   `json:downstream`
}

func getAllCountryTransmissions() [][]lineData {
	start := time.Now()
	defer fmt.Println("getAllCountryTransmissions: ", time.Since(start), " sec")

	container := make([][]lineData, 0)

	// ch := make(chan string)
	lineDataChannel := make(chan []lineData)

	result := ReadJsonEntsoe()

	datetimestring := "20.12.2022"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	urls := make([]string, 0)

	for k, _ := range result {
		urls = append(urls, getCountryTransmissions(datetimestring, k, timestamp))
	}

	var wg sync.WaitGroup
	wg.Add(len(urls))

	go func() {
		wg.Wait()
		close(lineDataChannel)
	}()

	c := colly.NewCollector()

	// Find and visit all links
	// c.OnHTML("table", func(e *colly.HTMLElement) {
	// 	e.DOM.Children().First().Find("tr>td").Parent().Each(func(_ int, s *goquery.Selection) {
	// 		symbol := s.Find("td a").First().Text()
	// 		fmt.Println(symbol)
	// 	})
	// 	e.Request.Visit(e.Attr("href"))
	// })

	c.OnHTML("table ", func(e *colly.HTMLElement) {
		defer wg.Done()
		schedule := make([]lineData, 0)
		upstreamDirection := make([]string, 2)
		downstreamDirection := make([]string, 2)
		e.ForEach("thead>tr>th", func(idx int, th *colly.HTMLElement) {
			txt := strings.Replace(th.Text, "\t", "", -1)
			txt = strings.Replace(txt, "\n", "", -1)
			txt = strings.Replace(txt, " ", "", -1)
			if idx == 0 {

			} else if idx == 1 {
				upstreamDirection = strings.Split(txt, ">")
				upstreamDirection[0] = upstreamDirection[0][len(upstreamDirection[0])-3 : len(upstreamDirection[0])-1]
				upstreamDirection[1] = upstreamDirection[1][len(upstreamDirection[1])-3 : len(upstreamDirection[1])-1]
			} else if idx == 2 {
				downstreamDirection = strings.Split(txt, ">")
				downstreamDirection[0] = downstreamDirection[0][len(downstreamDirection[0])-3 : len(downstreamDirection[0])-1]
				downstreamDirection[1] = downstreamDirection[1][len(downstreamDirection[1])-3 : len(downstreamDirection[1])-1]
			}
		})
		e.ForEach("tbody>tr", func(_ int, row *colly.HTMLElement) {
			lineDataEntry := lineData{}
			lineDataEntry.border = e.Request.URL.Query()["border.values"][0]
			lineDataEntry.upstreamDirection = upstreamDirection
			lineDataEntry.downstreamDirection = downstreamDirection
			row.ForEach("td", func(idx int, td *colly.HTMLElement) {
				txt := strings.Replace(td.Text, "\t", "", -1)
				txt = strings.Replace(txt, "\n", "", -1)
				txt = strings.Replace(txt, " ", "", -1)
				if idx == 0 {
					lineDataEntry.timeframe = txt
				} else if idx == 1 {
					lineDataEntry.upstream = txt
				} else {
					lineDataEntry.downstream = txt
				}
			})
			schedule = append(schedule, lineDataEntry)
		})
		lineDataChannel <- schedule
	})

	c.OnRequest(func(r *colly.Request) {
	})

	c.OnError(func(r *colly.Response, e error) {
		defer wg.Done()
		fmt.Println(wg)
	})
	c.OnResponse(func(r *colly.Response) {
		fmt.Println(wg)
	})

	for _, url := range urls {
		go c.Visit(url)
	}

	for x := range lineDataChannel {
		fmt.Println("Adding", x[0].border)
		container = append(container, x)
	}

	return container
}

func main() {
	port := os.Getenv("PORT")

	// After too many requests, user will get blocked by ENTSOE
	// results := getAllCountryTransmissions()
	// file, _ := json.MarshalIndent(results, "", " ")
	// _ = ioutil.WriteFile("test.json", file, 0644)
	// fmt.Println(results)

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	//router.Use(static.Serve("/", static.LocalFile("./dist/cryptoversum-frontend/", true)))
	//api := router.Group("/api")
	//{
	//	api.GET("/", func(c *gin.Context) {
	//		c.JSON(http.StatusOK, gin.H {
	//			"message": "pong",
	//		})
	//	})
	//}

	router.Run(":" + port)
}
