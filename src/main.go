package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"./database"
	"./docker"
	"github.com/orisano/uds"
)

const (
	dockerSocketPath = "/var/run/docker.sock"
)

var (
	dockerAPIClient = uds.NewClient(dockerSocketPath)
	//"time"
)

func main() {
	// server := &http.Server{
	// 	Addr:           ":8000",
	// 	Handler:        Handler{},
	// 	ReadTimeout:    10 * time.Second,
	// 	WriteTimeout:   10 * time.Second,
	// 	MaxHeaderBytes: 1 << 20,
	// }
	// server.ListenAndServe()
	var database = database.Database{}
	database.Connect()
	/*
		fmt.Print(time.Now())
		for index := 0; index < 100000; index++ {
			database.InsertFunction("Nome",2, 1024, "Código","Package")
			if index%1000==0{
				fmt.Println(index)
			}
		}
		fmt.Print(time.Now())
	*/

	fmt.Print(database.SelectAllFunction())
	database.Close()

	client := docker.Client{}
	client.Init()
	if isConnected := client.IsConnected(); !isConnected {
		fmt.Println("Failed to connect")
	}
	elapsedTime, err := client.CreateImage(
		"some:function",
		docker.FileInfo{Name: "Dockerfile", Text: "FROM ubuntu\n"},
	)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(elapsedTime)

	http.HandleFunc("/function", function)
	// http.HandleFunc("/metrics", metrics)
	// http.HandleFunc("/call", call)
	http.ListenAndServe(":8000", nil)
}

// Handler represents the http struct that hold a function to process requests.
// type Handler struct{}

// func (h Handler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
// 	res.Write([]byte(fmt.Sprintf("[%v] %v", req.Method, req.RequestURI)))
// }

func function(res http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		keys, ok := req.URL.Query()["name"]
		if ok && len(keys[0]) >= 1 {
			getFunctionByName(res, req, keys[0])
			return
		}
		getAllFunctions(res, req)
		return
	}

	if req.Body == nil {
		http.Error(res, "Empty body", 400)
		return
	}

	var body interface{}
	err := json.NewDecoder(req.Body).Decode(&body)
	if err != nil {
		http.Error(res, err.Error(), 400)
		return
	}

	var bodyData = body.(map[string]interface{})
	fmt.Println(bodyData["action"])
	fmt.Println(bodyData["name"])
	fmt.Println(bodyData["code"])
	fmt.Println(bodyData["package"])
	var containerOptions = bodyData["container-options"].(map[string]interface{})
	fmt.Println(containerOptions["cpus"])
	fmt.Println(containerOptions["memory"])

	resp, err := dockerAPIClient.Get("http://unix/images/json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(resp.Body)
	io.Copy(os.Stdout, resp.Body)
	resp.Body.Close()

	res.Write([]byte(fmt.Sprintf("[%v] %v\n", req.Method, req.RequestURI)))
}

func getAllFunctions(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf("[%v] %v\n", req.Method, req.RequestURI)))
}

func getFunctionByName(res http.ResponseWriter, req *http.Request, name string) {
	res.Write([]byte(fmt.Sprintf("Url Param 'param' found with value %v", name)))
}

func metrics(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf("[%v] %v\n", req.Method, req.RequestURI)))
}

func call(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf("[%v] %v\n", req.Method, req.RequestURI)))
}
