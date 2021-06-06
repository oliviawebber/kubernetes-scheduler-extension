package main

import (
	"fmt"
	"bytes"
	"context"
	"strings"
	"strconv"
	"net/http"
	"io"
	"encoding/json"
	"github.com/julienschmidt/httprouter"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	extender "k8s.io/kube-scheduler/extender/v1"
)

// Struct for PrioritizeMethod
type PrioritizeMethod struct {
	Name string
	Func func(pod v1.Pod, nodes []v1.Node) (*extender.HostPriorityList, error)
}

// Handler converts a pod and a list of nodes into a ordered list of host priorities
func (p PrioritizeMethod) Handler(args extender.ExtenderArgs) (*extender.HostPriorityList, error) {
	return p.Func(*args.Pod, args.Nodes.Items)
}

// ImagePriority defines the name and method for a priotity
// for each priority we should add a PrioritizeMethod
var ThermalPriority = PrioritizeMethod{
	Name: "thermal_prioritize",
	Func: func(pod v1.Pod, nodes []v1.Node) (*extender.HostPriorityList, error) {
		var priorityList extender.HostPriorityList
		priorityList = make([]extender.HostPriority, len(nodes))
		
		// Loop over every node, and get its thermal score
		for i, node := range nodes {
			score := int64(nodeThermalScore(node.Name))
			priorityList[i] = extender.HostPriority{
				Host:  node.Name,
				Score: score,
			}
		}
		return &priorityList, nil
	},
}

// Calculates the thermal score of a node by nodeName
// The score is simply 100 - <current node temperature>
// This gives the correct ordering i.e. cool nodes score higher than hotter nodes
// Potentially this score could be normalized somehow
func nodeThermalScore(nodeName string) int64 {
	metric := nodeThermalMetric(nodeName)
	return int64(100.0 - metric)
}

// Struct for FilterMethod
type FilterMethod struct {
	Name string
	Func func(pod v1.Pod, nodes []v1.Node) (*extender.ExtenderFilterResult, error)
}

// Given a list the handler filters out the nodes which are not valid
func (f FilterMethod) Handler(args extender.ExtenderArgs) (*extender.ExtenderFilterResult, error) {
	return f.Func(*args.Pod, args.Nodes.Items)
}

// Filters a list of nodes according to their thermal scores returning the ones which can be
// scheduled on
var ThermalFilter = FilterMethod{
	Name: "thermal_filter",
	Func: func(pod v1.Pod, nodes []v1.Node) (*extender.ExtenderFilterResult, error) {
		canSchedule := make([]v1.Node, 0, len(nodes))
		cannotSchedule := make(map[string]string)
		
		// For each node, if it exceeds a threshold value of 40, it cannot be scheduled
		for _, node := range nodes {
			if (nodeThermalMetric(node.Name) > 40) {
				cannotSchedule[node.Name] = "Too hot"
			} else {
				canSchedule = append(canSchedule, node)
			}
		}

		// Return a list of nodes that can and cannot be scheduled
		result := extender.ExtenderFilterResult{
			Nodes: &v1.NodeList{
				Items: canSchedule,
			},
			FailedNodes: cannotSchedule,
			Error: "",
		}

		return &result, nil
	},
}

// Given a nodeName fetches the nodes thermal temperature from the metrics server
func nodeThermalMetric(nodeName string) float64 {
	// Setup connection to the metrics server
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	// Fetch the metric of interest, in this case node_thermal_zone_temp
	path := "apis/custom.metrics.k8s.io/v1beta1/nodes/" + nodeName + "/node_thermal_zone_temp"
	data, _ := clientset.RESTClient().Get().AbsPath(path).DoRaw(context.TODO())

	// Convert the metric from Kubernetes format to a floating point
	dataString := string(data)
	startIndex := strings.Index(dataString, "value") + 8
	endIndex := strings.Index(dataString, "selector") - 4
	rawMetric, _ := strconv.ParseFloat(dataString[startIndex:endIndex],64)
	metric := rawMetric / 1000.0

	return metric
}

// Given a prioritizeMethod, setup the appropriate http endpoint
func prioritizeRoute(prioritizeMethod PrioritizeMethod) httprouter.Handle {
	return func(writer http.ResponseWriter, request *http.Request, p httprouter.Params) {
		var buffer bytes.Buffer
		body := io.TeeReader(request.Body, &buffer)

		var extenderArgs extender.ExtenderArgs

		json.NewDecoder(body).Decode(&extenderArgs)
		priorityList, _ := prioritizeMethod.Handler(extenderArgs)
		result, _ := json.Marshal(priorityList)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(result)
	}
}

// Given a filterMethod, setup the appropriate http endpoint
func filterRoute(filterMethod FilterMethod) httprouter.Handle {
	return func(writer http.ResponseWriter, request *http.Request, p httprouter.Params) {
		var buffer bytes.Buffer
		body := io.TeeReader(request.Body, &buffer)

		var extenderArgs extender.ExtenderArgs

		json.NewDecoder(body).Decode(&extenderArgs)
		priorityList, _ := filterMethod.Handler(extenderArgs)
		result, _ := json.Marshal(priorityList)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		writer.Write(result)
	}
}

func main() {
	router := httprouter.New()

	// Setup HTTP endpoints for the filter and prioritize endpoints
	prioritizePath := "/thermal_scheduler/prioritize/thermal_prioritize"
	router.POST(prioritizePath, prioritizeRoute(ThermalPriority))
	filterPath := "/thermal_scheduler/filter/thermal_filter"
	router.POST(filterPath, filterRoute(ThermalFilter))

	// Start Server, and listen on port 80
	fmt.Println("Starting server")
	if err := http.ListenAndServe(":80", router); err != nil {
		fmt.Println(err)
	}
	fmt.Println("Stopping server")
}
