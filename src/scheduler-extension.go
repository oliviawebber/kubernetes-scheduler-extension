package main

import (
	"flag"
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"strconv"
	"net/http"
	"io"
	"encoding/json"
	"github.com/julienschmidt/httprouter"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	//"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//schedulerapi "k8s.io/kubernetes/pkg/scheduler/apis/config/v1"
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
	Name: "thermal_score",
	Func: func(pod v1.Pod, nodes []v1.Node) (*extender.HostPriorityList, error) {
		var priorityList extender.HostPriorityList
		priorityList = make([]extender.HostPriority, len(nodes))
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

func nodeThermalScore(nodeName string) int64 {
	metric := nodeThermalMetric(nodeName)
	return int64(100.0 - metric)
}

func nodeThermalMetric(nodeName string) float64 {
	config, _ := rest.InClusterConfig()
	clientset, _ := kubernetes.NewForConfig(config)

	path := "apis/custom.metrics.k8s.io/v1beta1/nodes/" + nodeName + "/node_thermal_zone_temp"

	data, _ := clientset.RESTClient().Get().AbsPath(path).DoRaw(context.TODO())

	dataString := string(data)
	startIndex := strings.Index(dataString, "value") + 8
	endIndex := strings.Index(dataString, "selector") - 4
	rawMetric, _ := strconv.ParseFloat(dataString[startIndex:endIndex],64)
	metric := rawMetric / 1000.0

	return metric
}

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

func main() {
	router := httprouter.New()

	prioritizePath := "prioritize/thermal_score"
	router.POST(prioritizePath, prioritizeRoute(ThermalPriority))

	http.ListenAndServe("http://localhost:4321/thermalScheduler", router)
}
