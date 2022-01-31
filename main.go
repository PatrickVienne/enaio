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

type LineData struct {
	Border              string   `json:"border"`
	Date                string   `json:"date"`
	StartTimeStr        string   `json:"starttimestr"`
	EndTimeStr          string   `json:"endtimestr"`
	StartTime           int64    `json:"starttime"`
	EndTime             int64    `json:"endtime"`
	StartLat            float32  `json:"startLat"`
	StartLong           float32  `json:"startLong"`
	EndLat              float32  `json:"endLat"`
	EndLong             float32  `json:"endLong"`
	StartCCA3           string   `json:"startCCA3"`
	EndCCA3             string   `json:"endCCA3"`
	Timeframe           string   `json:"timeframe"`
	UpstreamDirection   []string `json:"upstreamDirection"`
	DownstreamDirection []string `json:"downstreamDirection"`
	Upstream            float64  `json:"upstream"`
	Downstream          float64  `json:"downstream"`
	NetStream           float64  `json:"netStream"`
}

func readFlow(yesterdayDate time.Time) []LineData {

	yesterday := fmt.Sprintf("%02d.%02d.%04d", yesterdayDate.Day(), yesterdayDate.Month(), yesterdayDate.Year())

	filename := yesterday + "_flow.json"

	var flows []LineData

	content, _ := ioutil.ReadFile(filename)
	json.Unmarshal(content, &flows)

	return flows
}

func readNet(yesterdayDate time.Time) map[int64]map[string]float64 {
	yesterday := fmt.Sprintf("%02d.%02d.%04d", yesterdayDate.Day(), yesterdayDate.Month(), yesterdayDate.Year())

	netfilename := yesterday + "_net.json"

	var netdata map[int64]map[string]float64

	content, _ := ioutil.ReadFile(netfilename)
	json.Unmarshal(content, &netdata)

	return netdata
}

func getAllCountryTransmissions(yesterdayDate time.Time, countryInfo map[string]CountryInfo) ([]LineData, map[int64]map[string]float64) {
	start := time.Now()
	defer fmt.Println("getAllCountryTransmissions: ", time.Since(start), " sec")

	container := make([]LineData, 0)

	// ch := make(chan string)
	lineDataChannel := make(chan []LineData)
	yesterday := fmt.Sprintf("%02d.%02d.%04d", yesterdayDate.Day(), yesterdayDate.Month(), yesterdayDate.Year())

	result := ReadJsonEntsoe()

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	urls := make([]string, 0)

	for k, _ := range result {
		urls = append(urls, getCountryTransmissions(yesterday, k, timestamp))
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
		schedule := make([]LineData, 0)
		startCca2 := ""
		endCca2 := ""
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
				startCca2 = upstreamDirection[0]
				endCca2 = upstreamDirection[1]
			} else if idx == 2 {
				downstreamDirection = strings.Split(txt, ">")
				downstreamDirection[0] = downstreamDirection[0][len(downstreamDirection[0])-3 : len(downstreamDirection[0])-1]
				downstreamDirection[1] = downstreamDirection[1][len(downstreamDirection[1])-3 : len(downstreamDirection[1])-1]
				startCca2 = downstreamDirection[1]
				endCca2 = downstreamDirection[0]
			}
		})
		if startCca2 == "" || endCca2 == "" {
			fmt.Println("Skipping", upstreamDirection, downstreamDirection)
			return
		}
		e.ForEach("tbody>tr", func(_ int, row *colly.HTMLElement) {
			lineDataEntry := LineData{}
			lineDataEntry.Date = yesterday
			lineDataEntry.Border = e.Request.URL.Query()["border.values"][0]
			lineDataEntry.UpstreamDirection = upstreamDirection
			lineDataEntry.DownstreamDirection = downstreamDirection

			if len(countryInfo[startCca2].LatLng) == 2 {
				lineDataEntry.StartLat = countryInfo[startCca2].LatLng[0]
				lineDataEntry.StartLong = countryInfo[startCca2].LatLng[1]
			} else {
				fmt.Println("Problem with", startCca2, countryInfo[startCca2])
			}

			if len(countryInfo[endCca2].LatLng) == 2 {
				lineDataEntry.EndLat = countryInfo[endCca2].LatLng[0]
				lineDataEntry.EndLong = countryInfo[endCca2].LatLng[1]
			} else {
				fmt.Println("Problem with", endCca2, countryInfo[endCca2])
			}
			lineDataEntry.StartCCA3 = countryInfo[startCca2].Cca3
			lineDataEntry.EndCCA3 = countryInfo[endCca2].Cca3

			row.ForEach("td", func(idx int, td *colly.HTMLElement) {
				txt := strings.Replace(td.Text, "\t", "", -1)
				txt = strings.Replace(txt, "\n", "", -1)
				txt = strings.Replace(txt, " ", "", -1)
				number, err := strconv.ParseFloat(txt, 64)
				if err != nil {
					number = 0
				}
				if idx == 0 {
					timeStrs := strings.Split(txt, "-")
					fromTimestamp, toTimestamp := dateTimeToTimestamp(yesterdayDate, txt)
					lineDataEntry.Timeframe = txt
					lineDataEntry.StartTime = fromTimestamp
					lineDataEntry.EndTime = toTimestamp
					lineDataEntry.StartTimeStr = timeStrs[0]
					lineDataEntry.EndTimeStr = timeStrs[1]
				} else if idx == 1 {
					lineDataEntry.Upstream = number
				} else {
					lineDataEntry.Downstream = number
				}
			})
			// total output of country
			// positive means country is a generator, negative means country is a consumer
			lineDataEntry.NetStream = lineDataEntry.Upstream - lineDataEntry.Downstream
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

	schulesByTime := make(map[int64][]LineData, 0)
	for x := range lineDataChannel {
		for _, entry := range x {
			container = append(container, entry)
			schulesByTime[entry.StartTime] = append(schulesByTime[entry.StartTime], entry)
		}
	}
	// TODO: reorganize, by time and country, list all target of country
	netByTimeAndCountry := make(map[int64]map[string]float64, 0)
	for timestamp, lines := range schulesByTime {
		for _, entryLine := range lines {

			if _, ok := netByTimeAndCountry[timestamp]; !ok {
				val := make(map[string]float64, 0)
				netByTimeAndCountry[timestamp] = val
			}

			if _, ok := netByTimeAndCountry[timestamp][entryLine.StartCCA3]; !ok {
				netByTimeAndCountry[timestamp][entryLine.StartCCA3] = 0
			}

			if _, ok := netByTimeAndCountry[timestamp][entryLine.EndCCA3]; !ok {
				netByTimeAndCountry[timestamp][entryLine.EndCCA3] = 0
			}

			netByTimeAndCountry[timestamp][entryLine.StartCCA3] = netByTimeAndCountry[timestamp][entryLine.StartCCA3] + entryLine.NetStream
			netByTimeAndCountry[timestamp][entryLine.EndCCA3] = netByTimeAndCountry[timestamp][entryLine.EndCCA3] - entryLine.NetStream
		}
	}

	return container, netByTimeAndCountry
}

type CountryCharge struct {
}

type CountryName struct {
	Common string `json:"common"`
}

type CountryInfo struct {
	Name   CountryName `json:"name"`
	Cca2   string      `json:"cca2"`
	Ccn3   string      `json:"ccn3"`
	Cca3   string      `json:"cca3"`
	Cioc   string      `json:"cioc"`
	LatLng []float32   `json:"latlng"`
}

func parseCountryInfo() map[string]CountryInfo {
	file, _ := ioutil.ReadFile("./static/countries_info.json")

	cca2ToInfo := make(map[string]CountryInfo, 0)

	data := []CountryInfo{}

	_ = json.Unmarshal([]byte(file), &data)
	for _, entry := range data {
		cca2ToInfo[entry.Cca2] = entry
	}
	return cca2ToInfo
}

func dateTimeToTimestamp(_date time.Time, _time string) (int64, int64) {
	times := strings.Split(_time, "-")
	start, _ := strconv.Atoi(times[0][:2])
	end, _ := strconv.Atoi(times[1][:2])
	cetLocation, _ := time.LoadLocation("CET")
	tm := time.Date(_date.Year(), _date.Month(), _date.Day(), start, 0, 0, 0, cetLocation).Unix()
	tm2 := time.Date(_date.Year(), _date.Month(), _date.Day(), end, 0, 0, 0, cetLocation).Unix()
	return tm, tm2
}

type JsonResult struct {
	Flows       []LineData
	Net         map[int64]map[string]float64
	CountryInfo map[string]CountryInfo
}

func main() {
	port := os.Getenv("PORT")

	countryInfo := parseCountryInfo()
	fmt.Println("Parsed  Country Info", len(countryInfo))

	// After too many requests, user will get blocked by ENTSOE
	yesterdayDate := time.Now().AddDate(0, 0, -1)
	// dayUnix := yesterdayDate.Unix() - (yesterdayDate.Unix() % (24 * 3600))
	// // yesterday := yesterdayDate.Format("20.12.2000")
	yesterday := fmt.Sprintf("%02d.%02d.%04d", yesterdayDate.Day(), yesterdayDate.Month(), yesterdayDate.Year())
	// time := "00:00-01:00"
	// fmt.Println(dateTimeToTimestamp(yesterdayDate, time))

	// fmt.Println("dayUnix", dayUnix)
	// fmt.Println(dateTimeToTimestamp(yesterdayDate, time))

	var results []LineData
	var netByTimeAndCountry map[int64]map[string]float64

	flowfilename := yesterday + "_flow.json"
	netfilename := yesterday + "_net.json"
	if _, err := os.Stat(flowfilename); err == nil {
		fmt.Printf("File exists: " + flowfilename + "\n")
		results = readFlow(yesterdayDate)
		netByTimeAndCountry = readNet(yesterdayDate)
	} else {
		results, netByTimeAndCountry = getAllCountryTransmissions(yesterdayDate, countryInfo)
		// file, _ := json.MarshalIndent(results, "", " ")
		flowfile, _ := json.Marshal(results)
		_ = ioutil.WriteFile(flowfilename, flowfile, 0644)
		fmt.Printf("File created: " + flowfilename + "\n")

		netfile, _ := json.Marshal(netByTimeAndCountry)
		_ = ioutil.WriteFile(netfilename, netfile, 0644)
		fmt.Printf("File created: " + netfilename + "\n")
	}

	if port == "" {
		// log.Fatal("$PORT must be set")
		log.Println("Setting Port to 8088")
		port = "8088"
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})
	router.GET("/api/total", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": JsonResult{results, netByTimeAndCountry, countryInfo}, "status": http.StatusOK})
	})
	router.GET("/api/countryInfo", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": countryInfo, "status": http.StatusOK})
	})
	router.GET("/api/schedules", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": results, "status": http.StatusOK})
	})
	router.GET("/api/flows", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": netByTimeAndCountry, "status": http.StatusOK})
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
