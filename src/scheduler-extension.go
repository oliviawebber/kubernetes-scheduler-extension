package thermalExtender
// Struct for PrioritizeMethod
type PrioritizeMethod struct {
	Name string
	Func func(pod v1.Pod, nodes []v1.Node) (*schedulingapi.HostPriorityList, error)
}

// Handler converts a pod and a list of nodes into a ordered list of host priorities
func (p PrioritizeMethod) Handler(args schedulingapi.ExtenderArgs) (*schedulingapi.HostPriorityList, error) {
	return p.Func(*args.Pod, args.Nodes.Items)
}

// ImagePriority defines the name and method for a priotity
// for each priority we should add a PrioritizeMethod
var ThermalPriority = PrioritizeMethod{
	Name: "thermal_score",
	Func: func(pod v1.Pod, nodes []v1.Node) (*schedulingapi.HostPriorityList, error) {
		var priorityList schedulingapi.HostPriorityList
		priorityList = make([]schedulingapi.HostPriority, len(nodes))
		for i, node := range nodes {
			score := nodeThermalScore(node.Name)
			priorityList[i] = schedulingapi.HostPriority{
				Host:  node.Name,
				Score: int(score),
			}
		}
		return &priorityList, nil
	},
}

func nodeThermalScore(pod v1.Pod, nodeImages)