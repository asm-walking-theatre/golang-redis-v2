package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/Jeffail/gabs"
	"github.com/gomodule/redigo/redis"
)

type Address struct {
	Latitude  float64 `json:"customer_latitude"`
	Longitude float64 `json:"customer_longitude"`
	Pincode   int     `json:"customer_pincode"`
}

type loc struct {
	Lat float64 `json: lat`
	Lon float64 `json: lon`
}

var Request_Count int = 1
var PincodeVersionMap = make(map[int]int)

func ResponseFromRedis(lat, long *float64) (int, error) {

	conn, err := redis.Dial("tcp", "localhost:6379")
	checkError(err)
	defer conn.Close()
	result := "-1"
	reply, err := conn.Do("GEORADIUS", "maps", *lat, *long, 1, "km", "ASC")

	if err != nil {
		return 1, err

	} else {

		vs := reply.([]interface{})

		for i, v := range vs {
			vs[i] = string(v.([]byte))
			if i == 0 {
				result = vs[i].(string)
				result = result[0:6]
			}
		}

		result_int, err := strconv.Atoi(result)
		checkError(err)
		return result_int, err
	}

}

func AddToRedis(lat, long *float64, pincode int) error {
	conn, err := redis.Dial("tcp", "localhost:6379")
	checkError(err)
	defer conn.Close()

	var pincode_v int = pincode*100 + PincodeVersionMap[pincode]
	PincodeVersionMap[pincode]++

	reply, err := conn.Do("GEOADD", "maps", *lat, *long, pincode_v)

	var CheckAdded int64 = 1

	if reply == CheckAdded {
		fmt.Printf("Successfully added lat:%v, long:%v for pincode: %v \n", lat, long, pincode)
	} else {
		fmt.Printf("new type of error")
	}

	return err
}

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func RequestToGoogleMapsAPI(lat, long *float64) int {

	var myurl string = "https://nominatim.openstreetmap.org/reverse?format=json&lat=" + fmt.Sprintf("%v", *lat) + "&lon=" + fmt.Sprintf("%v", *long)
	fmt.Println(myurl)
	response, err := http.Get(myurl)
	checkError(err)
	defer response.Body.Close()

	contentFromAPI, err := ioutil.ReadAll(response.Body)
	checkError(err)

	jsonParsed, err := gabs.ParseJSON(contentFromAPI)
	checkError(err)

	pincode := jsonParsed.Path("address.postcode").String()
	pincode = pincode[1:7]
	fmt.Println("reached here\n")
	pincode_int, err := strconv.Atoi(pincode)

	//checkError(err)
	if err != nil {
		fmt.Println("error converting to int", err)
	}

	return pincode_int
}

func CustomerHandler(w http.ResponseWriter, r *http.Request) {

	location := loc{}

	jsn, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Fatal("Error reading the body", err)
	}

	err = json.Unmarshal(jsn, &location)
	if err != nil {
		log.Fatal("Decoding error: ", err)
	}

	log.Printf("Received: %v\n", location)

	var customerAddress Address

	customerAddress.Latitude = location.Lat
	customerAddress.Longitude = location.Lon

	fmt.Println("Request Count : ", Request_Count)
	if Request_Count == 1 {
		//GenerateStructInRedis()
		FlushallInRedis()
		customerAddress.Pincode = RequestToGoogleMapsAPI(&location.Lat, &location.Lon)
		fmt.Println("Reached 1 \n")
		err := AddToRedis(&location.Lat, &location.Lon, customerAddress.Pincode)
		if err != nil {
			customerAddress.Pincode = RequestToGoogleMapsAPI(&location.Lat, &location.Lon)
		}
		//fmt.Println("Reached 2 \n")
		Request_Count++

	} else {
		fmt.Println("before entry \n")
		response, err := ResponseFromRedis(&location.Lat, &location.Lon)
		Request_Count++

		if err != nil {
			customerAddress.Pincode = RequestToGoogleMapsAPI(&location.Lat, &location.Lon)
		} else {

			if response == -1 {
				fmt.Println("after bringin response")
				customerAddress.Pincode = RequestToGoogleMapsAPI(&location.Lat, &location.Lon)
				err := AddToRedis(&location.Lat, &location.Lon, customerAddress.Pincode)
				if err != nil {
					customerAddress.Pincode = RequestToGoogleMapsAPI(&location.Lat, &location.Lon)
				}
			} else {
				customerAddress.Pincode = response
			}
		}
	}
	fmt.Println("Reached here as well")

	//fmt.Println("Request_Count : \n", Request_Count)
	customerAddressJson, err := json.Marshal(customerAddress)
	if err != nil {
		fmt.Fprintf(w, "Error: converting to JSOn %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(customerAddressJson)

}

func FlushallInRedis() {
	conn, err := redis.Dial("tcp", "localhost:6379")
	checkError(err)
	defer conn.Close()
	var i int = 5
	reply, err := conn.Do("FLUSHALL")
	if i == 3 {
		fmt.Println("Cant leave reply from redis", reply)
	}
	checkError(err)
}

func main() {
	http.HandleFunc("/", CustomerHandler)
	http.ListenAndServe(":8080", nil)

}
