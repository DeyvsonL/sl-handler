package main

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"./database"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var (
	db                            = database.Database{}
	mdb                           = database.NewMetricBD("../metrics.json")
	mdbMetricChan, mdbPersistChan = mdb.StartMetricDBRoutine()
	dockerClient                  = createDockerClient()
	dockerfile, _                 = ioutil.ReadFile("../dockerfiles/node/Dockerfile")
	serverJS, _                   = ioutil.ReadFile("../dockerfiles/node/server.js")
	serverStdioJS, _              = ioutil.ReadFile("../dockerfiles/node/server-stdio.js")
)

const (
	functionEndpoint = "/function/"
	metricsEndpoint  = "/metrics"
	callEndpoint     = "/call/"
	port             = ":8000"
)

func createDockerClient() *client.Client {

	// Create client
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}

	// Check API version
	_, err = cli.ImageList(context.TODO(), types.ImageListOptions{})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "client version") {
		cli.Close()
		errString := err.Error()
		supportedVersion := errString[strings.LastIndex(errString, " ")+1:]
		cliVersion := client.WithVersion(supportedVersion)
		cli, err = client.NewClientWithOpts(cliVersion)
	}

	// Check other errors
	if err != nil {
		panic(err)
	}

	return cli
}

func main() {
	db.Connect()

	http.HandleFunc(functionEndpoint, function)
	http.HandleFunc(metricsEndpoint, metrics)
	http.HandleFunc(callEndpoint, call)
	http.ListenAndServe(port, nil)
}

func function(res http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		functionGet(res, req)
	case "POST":
		functionPost(res, req)
	case "DELETE":
		functionDelete(res, req)
	default:
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func functionGet(res http.ResponseWriter, req *http.Request) {
	var argument = req.RequestURI[len(functionEndpoint):]
	if !strings.EqualFold(argument, "") {
		var function = functionGetByName(argument)
		if function == "" {
			res.Write([]byte(fmt.Sprintf("Function with name %v not found", argument)))
			res.WriteHeader(http.StatusNotFound)
			return
		}
		res.Write([]byte(function))

	} else {
		var functions = functionGetAll()
		res.Write([]byte(functions))
	}
}

func functionGetAll() string {
	return string(db.SelectAllFunction())
}

func functionGetByName(argument string) string {
	return string(db.SelectFunction(argument))
}

func functionPost(res http.ResponseWriter, req *http.Request) {
	name, memory, code, pack := ExtractFunction(res, req.Body)
	if len(db.SelectFunction(name)) == 0 {
		tarBuffer := CreateTarReader(
			FileInfo{Name: "Dockerfile", Text: string(dockerfile)},
			FileInfo{Name: "server.js", Text: string(serverStdioJS)},
			FileInfo{Name: "package.json", Text: pack},
			FileInfo{Name: "code.js", Text: code},
		)
		buildResponse, err := dockerClient.ImageBuild(
			context.TODO(),
			tarBuffer,
			types.ImageBuildOptions{Tags: []string{name}},
		)
		io.Copy(os.Stdout, buildResponse.Body)
		fmt.Print(buildResponse, err)
		db.InsertFunction(name, memory, code, pack)
		var function = functionGetByName(name)
		res.Write([]byte(function))
		res.Write([]byte(fmt.Sprintf("Function Created at %v%v\n", req.RequestURI, name)))
		res.WriteHeader(http.StatusCreated)
	} else {
		http.Error(res, "Function already exist\n"+http.StatusText(http.StatusConflict), http.StatusConflict)
	}
}

func ExtractFunction(res http.ResponseWriter, jsonBodyReq io.Reader) (name string, memory int, code, pack string) {
	var jsonBody interface{}
	err := json.NewDecoder(jsonBodyReq).Decode(&jsonBody)
	if err != nil {
		http.Error(res, err.Error(), 400)
		return
	}

	var bodyData = jsonBody.(map[string]interface{})
	return bodyData["name"].(string), int(bodyData["memory"].(float64)), bodyData["code"].(string), bodyData["package"].(string)
}

func functionDelete(res http.ResponseWriter, req *http.Request) {
	var name = strings.Split(req.RequestURI, "/")[2]

	if len(db.SelectFunction(name)) > 0 {
		dockerClient.ImageRemove(context.Background(), name, types.ImageRemoveOptions{})
		var sucess = db.DeleteFunction(name)
		if !sucess {
			res.Write([]byte(fmt.Sprintf("Cannot Delete function %v\n", name)))
			res.WriteHeader(http.StatusBadRequest)
			return
		}

		res.Write([]byte(fmt.Sprintf("Function Deleted [%v] %v\n", req.Method, req.RequestURI)))
		res.WriteHeader(http.StatusNoContent)
	} else {
		http.Error(res, "Function don't exist\n"+http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func metrics(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(fmt.Sprintf("[%v] %v\n", req.Method, req.RequestURI)))
}

func call(res http.ResponseWriter, req *http.Request) {
	var startTime time.Time

	startTime = time.Now()
	function, endpoint, queryMap := SplitFunctionUrl(req.RequestURI, len(callEndpoint))
	method := req.Method
	headers := req.Header
	queryJSON, _ := json.Marshal(queryMap)
	headersJSON, _ := json.Marshal(headers)
	fmt.Printf("## Url + Headers Parse Time: %v\n", time.Since(startTime))

	startTime = time.Now()
	createResponse, _ := dockerClient.ContainerCreate(
		context.TODO(),
		&container.Config{Image: function, Cmd: []string{"node", "server.js", endpoint, string(queryJSON), method, string(headersJSON)}, AttachStdout: true, Tty: true},
		&container.HostConfig{},
		&network.NetworkingConfig{},
		"",
	)
	containerID := createResponse.ID
	fmt.Printf("## Container Create Time: %v\n", time.Since(startTime))
	fmt.Printf("## Container ID: %v\n", containerID)

	startTime = time.Now()
	dockerClient.ContainerStart(context.TODO(), containerID, types.ContainerStartOptions{})
	fmt.Printf("## Container Start Time: %v\n", time.Since(startTime))

	// fmt.Printf("## Container IP: %v\n", containerIP)

	// startApplicationConnectionTime := time.Now()
	// var applicationRunTime time.Duration
	// gatewayReq, err := http.NewRequest(req.Method, fmt.Sprintf("http://%v:8080/%v", containerIP, requestData[len(imageName)+1:]), req.Body)
	// var gatewayRes *http.Response
	// for i := 0; i < 200; i++ {
	// 	fmt.Printf("Connection tries: %v\n", i)
	// 	startApplicationRunTime := time.Now()
	// 	gatewayRes, err = http.DefaultClient.Do(gatewayReq)
	// 	fmt.Println(err)
	// 	if err == nil {
	// 		applicationRunTime = time.Since(startApplicationRunTime)
	// 		fmt.Printf("## Request Run Time: %v\n", applicationRunTime)
	// 		fmt.Println("Success!")
	// 		break
	// 	}p
	// 	time.Sleep(10 * time.Millisecond)
	// }
	// applicationConnectionTime := time.Since(startApplicationConnectionTime)
	// fmt.Printf("## Request Time: %v\n", applicationConnectionTime)

	startTime = time.Now()
	dockerClient.ContainerWait(context.TODO(), containerID, container.WaitConditionNotRunning)
	fmt.Printf("## Container Wait Stop Time: %v\n", time.Since(startTime))

	startTime = time.Now()
	logResponseReader, _ := dockerClient.ContainerLogs(context.TODO(), containerID, types.ContainerLogsOptions{ShowStdout: true, Follow: true, Tail: "1"})
	logHeader := make([]byte, 8)
	logResponseReader.Read(logHeader)
	logResponse, _ := ioutil.ReadAll(logResponseReader)
	functionResponse := logResponse[bytes.LastIndexByte(logResponse, byte('\n'))+1:]
	logResponseReader.Close()
	fmt.Println(string(functionResponse))
	fmt.Printf("## Container Get Logs Time: %v\n", time.Since(startTime))

	startTime = time.Now()
	var functionResult map[string]interface{}
	json.Unmarshal(functionResponse, &functionResult)
	functionCode := int(functionResult["code"].(float64))
	functionBody, _ := functionResult["body"]
	// functionHeaders := functionResult["headers"].(map[string]string)
	res.WriteHeader(functionCode)
	functionBodyBytes, _ := json.Marshal(functionBody)
	res.Write(functionBodyBytes)
	fmt.Printf("## Process Function And Respond Time: %v\n", time.Since(startTime))

	startTime = time.Now()
	dockerClient.ContainerRemove(context.Background(), containerID, types.ContainerRemoveOptions{})
	fmt.Printf("## Remove Container Time: %v\n", time.Since(startTime))
	// fmt.Println(client.DeleteImage(imageName))

	// metric := database.Metric{
	// 	Function:                  imageName,
	// 	ContainerID:               containerID,
	// 	ContainerCreateTime:       containerCreateTime,
	// 	ContainerStartTime:        containerStartTime,
	// 	ApplicationConnectionTime: applicationConnectionTime,
	// 	ApplicationRunTime:        applicationRunTime,
	// 	ApplicationCode:           applicationCode,
	// 	ContainerStopTime:         containerStopTime,
	// 	ContainerDeleteTime:       containerDeleteTime,
	// }

	// mdbPersistChan <- true // disable later
	// mdbMetricChan <- metric
}

// func serialize() {
// 	   mdbPersistChan <- true
// 	   mdbMetricChan <- database.Metric{}
// }

type FileInfo struct {
	Name string
	Text string
}

func CreateTarReader(files ...FileInfo) io.Reader {
	tarBuffer := bytes.Buffer{}
	tarWriter := tar.NewWriter(&tarBuffer)
	for _, file := range files {
		tarHeader := &tar.Header{Name: file.Name, Mode: 0600, Size: int64(len(file.Text))}
		tarWriter.WriteHeader(tarHeader)
		tarWriter.Write([]byte(file.Text))
	}
	tarWriter.Close()
	return bytes.NewReader(tarBuffer.Bytes())
}

func SplitFunctionUrl(url string, prefixSize int) (string, string, map[string]string) {
	url = url[prefixSize:]
	slash := strings.Index(url, "/")
	if slash == -1 {
		return "", "", make(map[string]string)
	}
	questionMark := strings.Index(url, "?")
	if questionMark == -1 {
		questionMark = len(url)
	}

	function := url[:slash]
	endpoint := url[slash+1 : questionMark]
	queryMap := make(map[string]string)

	if questionMark == len(url) {
		return function, endpoint, queryMap
	}

	query := url[questionMark:]
	queryParts := strings.Split(query, "&")
	for _, queryPart := range queryParts {
		equals := strings.Index(queryPart, "=")
		queryMap[queryPart[:equals]] = queryPart[equals+1:]
	}
	return function, endpoint, queryMap
}
