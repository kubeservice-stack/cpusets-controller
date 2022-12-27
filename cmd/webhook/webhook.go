package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kubeservice-stack/common/pkg/logger"
	"github.com/kubeservice-stack/cpusets-controller/pkg/config"
	"github.com/kubeservice-stack/cpusets-controller/pkg/types"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	//QuotaAll is one of the possible values of the webhook cfs-quotas parameter. All means CFS quotas should be automatically provisioned for every container
	QuotaAll = "all"
	//QuotaShared is one of the possible values of the webhook cfs-quotas parameter. Shared means CFS quotas should be automatically provisioned only for shared user containers
	QuotaShared = "shared"
)

var (
	scheme             = runtime.NewScheme()
	codecs             = serializer.NewCodecFactory(scheme)
	resourceBaseName   = "cmss.cn"
	processStarterPath = "/opt/bin/process-starter"
	certFile           string
	keyFile            string
	cfsQuotas          string
	mainLogger         = logger.GetLogger("cmd/webhook", "main")
)

type containerPoolRequests struct {
	sharedCPURequests    int
	exclusiveCPURequests int
	pools                map[string]int
}

type poolRequestMap map[string]containerPoolRequests

type patch struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
		Allowed: false,
	}
}

func getCPUPoolRequests(pod *corev1.Pod) (poolRequestMap, error) {
	var poolRequests = make(poolRequestMap)
	for _, c := range pod.Spec.Containers {
		cPoolRequests, exists := poolRequests[c.Name]
		if !exists {
			cPoolRequests.pools = make(map[string]int)
		}
		for key, value := range c.Resources.Limits {
			if strings.HasPrefix(string(key), resourceBaseName) {
				//convert back from human readable format
				val, err := strconv.Atoi(strings.Replace(value.String(), "k", "000", 1))
				if err != nil {
					mainLogger.Error("Cannot convert cpu request to int", logger.Any("key", key), logger.Any("value", value))
					return poolRequestMap{}, err
				}
				if strings.HasPrefix(string(key), resourceBaseName+"/shared") {
					cPoolRequests.sharedCPURequests += val
				}
				if strings.HasPrefix(string(key), resourceBaseName+"/exclusive") {
					cPoolRequests.exclusiveCPURequests += val
				}
				poolName := strings.TrimPrefix(string(key), resourceBaseName+"/")
				cPoolRequests.pools[poolName] = val
				poolRequests[c.Name] = cPoolRequests
			}
		}
	}
	return poolRequests, nil
}

func annotationNameFromConfig() string {
	return resourceBaseName + "/cpus"

}

func validateAnnotation(poolRequests poolRequestMap, cpuAnnotation types.CPUAnnotation) error {
	for _, cName := range cpuAnnotation.ContainerNames() {
		for _, pool := range cpuAnnotation.ContainerPools(cName) {
			cPoolRequests, exists := poolRequests[cName]
			if !exists {
				return fmt.Errorf("Container %s has no pool requests in pod spec",
					cName)
			}
			if cpuAnnotation.ContainerSharedCPUTime(cName) != cPoolRequests.sharedCPURequests {
				return fmt.Errorf("Shared CPU requests %d do not match to annotation %d",
					cPoolRequests.sharedCPURequests,
					cpuAnnotation.ContainerSharedCPUTime(cName))
			}
			value, exists := cPoolRequests.pools[pool]
			if !exists {
				return fmt.Errorf("Container %s; Pool %s in annotation not found from resources", cName, pool)
			}
			// cpu request in annotation can be twice as exclusive pool request in resources in case of HT is enabled (HT policy is "multiThreaded")
			if cpuAnnotation.ContainerTotalCPURequest(pool, cName) > 2*value {
				return fmt.Errorf("Exclusive CPU requests %d do not match to annotation %d",
					cPoolRequests.pools[pool],
					cpuAnnotation.ContainerTotalCPURequest(pool, cName))
			}
		}
	}
	return nil
}

func setRequestLimit(requests containerPoolRequests, patchList []patch, contID int, contSpec *corev1.Container) []patch {
	totalCFSLimit := 0
	if requests.exclusiveCPURequests > 0 && cfsQuotas == QuotaAll {
		if requests.sharedCPURequests > 0 {
			//This is the case when both shared, and exclusive pool resources are requested by the same container
			//To avoid artificially throttling the exclusive user threads when the shared threads are overstepping their boundaries,
			// we need to include the full size of the shared pool into the limit calculation.
			//This unfortunately allows mixed users to overstep their boundaries, but is the only way to ensure shared threads cannot
			// throttle the latency sensitive ones with their occasional bursts.
			//#PerformanceFirst
			totalCFSLimit = 1000*requests.exclusiveCPURequests + getMaxSharedPoolLimit(requests, contSpec)
		} else {
			//When only exclusive CPUs are requested we pad the limits with an arbitrary margin to avoid accidentally throttling sensitive workloads
			totalCFSLimit = 1000*requests.exclusiveCPURequests + 100
		}
	} else if requests.sharedCPURequests > 0 {
		totalCFSLimit = requests.sharedCPURequests
	}
	if totalCFSLimit > 0 {
		patchList = patchCPULimit(totalCFSLimit, patchList, contID, contSpec)
	}
	return patchList
}

func getMaxSharedPoolLimit(requests containerPoolRequests, contSpec *corev1.Container) int {
	poolConfs, err := types.ReadAllPoolConfigs(config.FileMatch)
	if err != nil {
		mainLogger.Warn("Container " + contSpec.Name + " asked for mixed allocations but pool configs could not be read to determine proper CFS limit - only exclusive allocations are accounted for properly")
		return requests.sharedCPURequests
	}
	var sharedPoolName string
	for poolName, request := range requests.pools {
		if request == requests.sharedCPURequests {
			sharedPoolName = poolName
		}
	}
	maxSharedPoolSize := 0
	for _, poolConf := range poolConfs {
		if pool, ok := poolConf.Pools[sharedPoolName]; ok {
			if pool.CPUset.Size()*1000 > maxSharedPoolSize {
				maxSharedPoolSize = pool.CPUset.Size() * 1000
			}
		}
	}
	return maxSharedPoolSize
}

func patchCPULimit(sharedCPUTime int, patchList []patch, i int, c *corev1.Container) []patch {
	var patchItem patch

	patchItem.Op = "replace"
	cpuVal := `"` + strconv.Itoa(sharedCPUTime) + `m"`
	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/resources/limits/cpu"
	patchItem.Value = json.RawMessage(cpuVal)
	patchList = append(patchList, patchItem)

	patchItem.Op = "replace"
	cpuVal = `"0m"`
	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/resources/requests/cpu"
	patchItem.Value = json.RawMessage(cpuVal)
	patchList = append(patchList, patchItem)

	return patchList
}

func patchContainerEnv(poolRequests poolRequestMap, envPatched bool, patchList []patch, i int, c *corev1.Container) ([]patch, error) {
	var patchItem patch
	var poolStr string

	for _, envVar := range c.Env {
		if envVar.Name == "CPU_POOLS" {
			// Don't reapply already patched item
			return patchList, nil
		}
	}

	if poolRequests[c.Name].exclusiveCPURequests > 0 && poolRequests[c.Name].sharedCPURequests > 0 {
		poolStr = types.ExclusivePoolID + "&" + types.SharedPoolID
	} else if poolRequests[c.Name].exclusiveCPURequests > 0 {
		poolStr = types.ExclusivePoolID
	} else if poolRequests[c.Name].sharedCPURequests > 0 {
		poolStr = types.SharedPoolID
	} else {
		poolStr = types.DefaultPoolID
	}
	patchItem.Op = "add"
	cpuPoolEnvPatch := `{"name":"CPU_POOLS","value":"` + poolStr + `" }`
	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/env"
	if envPatched || len(c.Env) > 0 {
		patchItem.Path += "/-"
	} else {
		cpuPoolEnvPatch = `[` + cpuPoolEnvPatch + `]`
	}
	patchItem.Value = json.RawMessage(cpuPoolEnvPatch)
	patchList = append(patchList, patchItem)

	return patchList, nil
}

func patchContainerForPinning(cpuAnnotation types.CPUAnnotation, patchList []patch, i int, c *corev1.Container) ([]patch, error) {
	var patchItem patch

	for _, volMount := range c.VolumeMounts {
		if volMount.Name == "podinfo" {
			// Assuming that all other container related patch is already patched, skip reapply these patches
			return patchList, nil
		}
	}

	mainLogger.Info("Adding CPU pinning patches")
	// podinfo volumeMount
	patchItem.Op = "add"

	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/volumeMounts/-"
	patchItem.Value =
		json.RawMessage(`{"name":"podinfo","mountPath":"/etc/podinfo","readOnly":true}`)
	patchList = append(patchList, patchItem)

	// hostbin volumeMount. Location for process starter binary

	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/volumeMounts/-"
	contVolumePatch := `{"name":"hostbin","mountPath":"` + processStarterPath + `","readOnly":true}`
	patchItem.Value =
		json.RawMessage(contVolumePatch)
	patchList = append(patchList, patchItem)

	// Container name to env variable
	contNameEnvPatch := `{"name":"CONTAINER_NAME","value":"` + c.Name + `" }`
	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/env"
	if len(c.Env) > 0 {
		patchItem.Path += "/-"
	} else {
		contNameEnvPatch = `[` + contNameEnvPatch + `]`
	}
	patchItem.Value = json.RawMessage(contNameEnvPatch)
	patchList = append(patchList, patchItem)

	// Overwrite entrypoint
	patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/command"
	contEPPatch := `[ "` + processStarterPath + `" ]`
	patchItem.Value = json.RawMessage(contEPPatch)
	patchList = append(patchList, patchItem)

	// Put command to args if pod cpu annotation does not exist for the container
	if len(c.Command) > 0 && !cpuAnnotation.IsContainerExists(c.Name) {
		patchItem.Path = "/spec/containers/" + strconv.Itoa(i) + "/args"
		args := `[ "` + strings.Join(c.Command, "\",\"") + `" `
		if len(c.Args) > 0 {
			args += `,"` + strings.Join(c.Args, "\",\"") + `"`
		}
		args += `]`
		patchItem.Value = json.RawMessage(args)
		patchList = append(patchList, patchItem)
	}
	return patchList, nil
}

func patchVolumesForPinning(patchList []patch) []patch {
	var patchItem patch
	patchItem.Op = "add"

	// podinfo volume
	patchItem.Path = "/spec/volumes/-"
	patchItem.Value = json.RawMessage(`{"name":"podinfo","downwardAPI": { "items": [ { "path" : "annotations","fieldRef":{ "fieldPath": "metadata.annotations"} } ] } }`)
	patchList = append(patchList, patchItem)
	// hostbin volume
	patchItem.Path = "/spec/volumes/-"
	volumePathPatch := `{"name":"hostbin","hostPath":{ "path":"` + processStarterPath + `"} }`
	patchItem.Value = json.RawMessage(volumePathPatch)
	patchList = append(patchList, patchItem)
	return patchList
}

func mutatePods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	mainLogger.Info("mutating pods")
	var (
		patchList         []patch
		err               error
		cpuAnnotation     types.CPUAnnotation
		pinningPatchAdded bool
	)

	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		mainLogger.Error("expect resource to be", logger.Any("resource", podResource))
		return nil
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err = deserializer.Decode(raw, nil, &pod); err != nil {
		mainLogger.Error("deserializer Decode error!", logger.Error(err))
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}

	annotationName := annotationNameFromConfig()

	reviewResponse.Allowed = true

	podAnnotation, podAnnotationExists := pod.ObjectMeta.Annotations[annotationName]

	poolRequests, err := getCPUPoolRequests(&pod)
	if err != nil {
		mainLogger.Error("Failed to get pod cpu pool requests", logger.Error(err))
		return toAdmissionResponse(err)
	}

	if podAnnotationExists {
		cpuAnnotation = types.NewCPUAnnotation()

		err = cpuAnnotation.Decode([]byte(podAnnotation))
		if err != nil {
			mainLogger.Error("Failed to decode pod annotation ", logger.Error(err))
			return toAdmissionResponse(err)
		}
		if err = validateAnnotation(poolRequests, cpuAnnotation); err != nil {
			mainLogger.Error("validateAnnotation error ", logger.Error(err))
			return toAdmissionResponse(err)
		}
	}

	// Patch container if needed.
	for contID, contSpec := range pod.Spec.Containers {
		patchList = setRequestLimit(poolRequests[contSpec.Name], patchList, contID, &contSpec)
		// If pod annotation has entry for this container or
		// container asks for exclusive cpus, we add patches to enable pinning.
		// The patches enable process in container to be started with cpu pooler's 'process starter'
		// The cpusetter sets cpuset for the container and that needs to be completed
		// before application container is started. If cpuset is set after the application
		// has started, the cpu affinity setting by application will be overwritten by the cpuset.
		// The process starter will wait for cpusetter to finish it's job for this container
		// and starts the application process after that.
		pinningPatchNeeded := cpuAnnotation.IsContainerExists(contSpec.Name)
		if poolRequests[contSpec.Name].exclusiveCPURequests > 0 {
			if len(contSpec.Command) == 0 && !pinningPatchNeeded {
				mainLogger.Warn("Container " + contSpec.Name + " asked exclusive cpus but command not given. CPU affinity settings possibly lost for container")
			} else {
				pinningPatchNeeded = true
			}
		}
		containerEnvPatched := false
		if pinningPatchNeeded {
			mainLogger.Info("Patch container for pinning " + contSpec.Name)

			patchList, err = patchContainerForPinning(cpuAnnotation, patchList, contID, &contSpec)
			if err != nil {
				return toAdmissionResponse(err)
			}
			pinningPatchAdded = true
			containerEnvPatched = true
		}
		if poolRequests[contSpec.Name].sharedCPURequests > 0 ||
			poolRequests[contSpec.Name].exclusiveCPURequests > 0 {
			// Patch container environment variable
			patchList, err = patchContainerEnv(poolRequests, containerEnvPatched, patchList, contID, &contSpec)
			if err != nil {
				return toAdmissionResponse(err)
			}
		}
	}
	// Add volumes if any container was patched for pinning
	if pinningPatchAdded {
		exists := false
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == "hostbin" || volume.Name == "podinfo" {
				exists = true
			}
		}
		if !exists {
			patchList = patchVolumesForPinning(patchList)
		}
	} else if podAnnotationExists {
		mainLogger.Error("CPU annotation exists but no container was patched", logger.Any("annotation", cpuAnnotation), logger.Any("containers", pod.Spec.Containers))
		return toAdmissionResponse(errors.New("CPU Annotation error"))
	}

	if len(patchList) > 0 {
		patch, err := json.Marshal(patchList)
		if err != nil {
			mainLogger.Error("Patch marshall error", logger.Error(err), logger.Any("patchlist", patchList))
			reviewResponse.Allowed = false
			return toAdmissionResponse(err)
		}
		reviewResponse.Patch = []byte(patch)
		pt := v1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &pt
	}

	return &reviewResponse
}

func serveMutatePod(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		mainLogger.Error("contentType is not application/json", logger.Any("contentType", contentType))
		return
	}

	requestedAdmissionReview := v1beta1.AdmissionReview{}

	responseAdmissionReview := v1beta1.AdmissionReview{}

	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		mainLogger.Error("deserializer decode error", logger.Error(err))
		responseAdmissionReview.Response = toAdmissionResponse(err)
	} else {
		responseAdmissionReview.Response = mutatePods(requestedAdmissionReview)
	}

	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID

	respBytes, err := json.Marshal(responseAdmissionReview)

	if err != nil {
		mainLogger.Error("json Marshal error", logger.Error(err))
	}
	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write(respBytes); err != nil {
		mainLogger.Error("respose write error", logger.Error(err))
	}
}

func main() {
	flag.StringVar(&certFile, "tls-cert-file", certFile, ""+
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated "+
		"after server cert).")
	flag.StringVar(&keyFile, "tls-private-key-file", keyFile, ""+
		"File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&processStarterPath, "process-starter-path", processStarterPath, ""+
		"Path to process-starter binary file. Optional parameter, default path is /opt/bin/process-starter.")
	flag.StringVar(&cfsQuotas, "cfs-quotas", QuotaAll,
		"Controls if CPUSets automatically provisions CFS quotas for its managed containers.\n"+
			"Possible values are:\n"+
			"'all'    - CPUSets provisions CFS quotas for all containers\n"+
			"'shared' - CPUSets doesn't provision quotas for containers using exclusive pools")
	flag.Parse()

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		mainLogger.Error("tls LoadX509KeyPair error", logger.Error(err))
	}

	http.HandleFunc("/mutating", serveMutatePod)
	server := &http.Server{
		Addr:         ":443",
		TLSConfig:    &tls.Config{Certificates: []tls.Certificate{cert}},
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	server.ListenAndServeTLS("", "")
}
