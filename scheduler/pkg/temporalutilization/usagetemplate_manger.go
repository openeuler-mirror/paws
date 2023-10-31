package temporalutilization

import (
	"fmt"
	"sync"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	pawsclientset "gitee.com/openeuler/paws/scheduler/pkg/generated/clientset/versioned"
	pawsInformer "gitee.com/openeuler/paws/scheduler/pkg/generated/informers/externalversions/scheduling/v1alpha1"
	pawslister "gitee.com/openeuler/paws/scheduler/pkg/generated/listers/scheduling/v1alpha1"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/utils"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informerv1 "k8s.io/client-go/informers/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	clientcache "k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

type UsageTemplateManager struct {
	// pawsClient is paws-specific client
	pawsClient pawsclientset.Interface
	// snapshotSharedLister is a lister for k8s object
	snapshotSharedLister framework.SharedLister
	// utLister is a UsageTemplate lister
	utLister pawslister.UsageTemplateLister
	// podLister is a pod lister
	podLister listerv1.PodLister

	// NodePodsCache stores pods that are scheduled/reserved on the corresponding node
	NodePodsCache map[string][]NamespacedPod
	sync.RWMutex
}

func NewUsageTemplateManager(pawsclient pawsclientset.Interface, snapshotSharedLister framework.SharedLister, utInformer pawsInformer.UsageTemplateInformer, podInformer informerv1.PodInformer) *UsageTemplateManager {

	utMgr := &UsageTemplateManager{
		pawsClient:           pawsclient,
		snapshotSharedLister: snapshotSharedLister,
		utLister:             utInformer.Lister(),
		podLister:            podInformer.Lister(),
		NodePodsCache:        make(map[string][]NamespacedPod),
	}

	utMgr.AddEventHandler(podInformer.Informer())

	return utMgr
}

// GetUsageTemplate returns the Usage Template that a pod belongs to.
func (utMgr *UsageTemplateManager) GetUsageTemplate(pod *corev1.Pod) (string, *v1alpha1.UsageTemplate) {
	utName := utils.GetUsageTemplateLabel(pod)
	if len(utName) == 0 {
		return "", nil
	}

	namespace := pod.GetNamespace()
	if len(namespace) == 0 {
		namespace = "default"
	}

	ut, err := utMgr.utLister.UsageTemplates(namespace).Get(utName)
	if err != nil {
		return fmt.Sprintf("%v/%v", namespace, utName), nil
	}

	return fmt.Sprintf("%v/%v", namespace, utName), ut
}

func (utMgr *UsageTemplateManager) OnAdd(obj interface{}) {
	pod := obj.(*corev1.Pod)
	utMgr.updateCache(nil, pod)
}

func (utMgr *UsageTemplateManager) OnDelete(obj interface{}) {
	pod := obj.(*corev1.Pod)
	nodeName := pod.Spec.NodeName
	utMgr.deleteFromCacheIfExists(pod, nodeName)
}

func (utMgr *UsageTemplateManager) deleteFromCacheIfExists(pod *corev1.Pod, nodeName string) {
	utMgr.Lock()
	defer utMgr.Unlock()
	if _, ok := utMgr.NodePodsCache[nodeName]; !ok {
		// Probably deleted, as node name is not found in the cache
		klog.V(8).InfoS("node not found", "node", nodeName, "pod", pod.Name, "namespace", pod.Namespace)
		return
	}

	// shift and delete
	for i := 0; i < len(utMgr.NodePodsCache[nodeName]); i++ {
		if utMgr.NodePodsCache[nodeName][i].Name == pod.Name && utMgr.NodePodsCache[nodeName][i].Namespace == pod.Namespace {
			utMgr.NodePodsCache[nodeName] = append(utMgr.NodePodsCache[nodeName][:i], utMgr.NodePodsCache[nodeName][i+1:]...)
		}
	}

	if len(utMgr.NodePodsCache[nodeName]) == 0 {
		delete(utMgr.NodePodsCache, nodeName)
	}
}

func (utMgr *UsageTemplateManager) addToCacheIfNotExists(pod *corev1.Pod, nodeName string) {
	utMgr.Lock()
	defer utMgr.Unlock()
	// store it in the nodePodsCache for scoring
	// TODO: check whether memory leaks happens if pod swifted from node to node
	if _, ok := utMgr.NodePodsCache[nodeName]; !ok {
		utMgr.NodePodsCache[nodeName] = make([]NamespacedPod, 0)
	}

	// if the cache already contains the pod , safe to ignore
	for _, namespacedPod := range utMgr.NodePodsCache[nodeName] {
		if namespacedPod.Name == pod.Name && namespacedPod.Namespace == pod.Namespace {
			return
		}
	}

	utMgr.NodePodsCache[nodeName] = append(utMgr.NodePodsCache[nodeName], NamespacedPod{Namespace: pod.Namespace, Name: pod.Name})
}

// updateCache update the internal cache that track where pods are on which node
func (utMgr *UsageTemplateManager) updateCache(old, pod *corev1.Pod) {
	if pod.Spec.NodeName == "" {
		return
	}

	// delete old pod cache just incase
	if old != nil {
		utMgr.OnDelete(old)
	}

	utMgr.addToCacheIfNotExists(pod, pod.Spec.NodeName)
}

func (utMgr *UsageTemplateManager) OnUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	newPod := newObj.(*corev1.Pod)

	// if node name changes
	if oldPod.Spec.NodeName != newPod.Spec.NodeName {
		utMgr.updateCache(oldPod, newPod)
		return
	}

	// if UsageTemplate CRD name changed
	if utils.GetUsageTemplateLabel(oldPod) != utils.GetUsageTemplateLabel(newPod) {
		utMgr.updateCache(oldPod, newPod)
		return
	}
}

func (utMgr *UsageTemplateManager) AddEventHandler(informer clientcache.SharedIndexInformer) {
	informer.AddEventHandler(
		clientcache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *corev1.Pod:
					return isAssigned(t)
				case clientcache.DeletedFinalStateUnknown:
					if pod, ok := t.Obj.(*corev1.Pod); ok {
						return isAssigned(pod)
					}
					utilruntime.HandleError(fmt.Errorf("unable to conver object %T to *v1.Pod", obj))
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object: %T", obj))
					return false
				}

			},
			Handler: utMgr,
		},
	)
}

func isAssigned(pod *corev1.Pod) bool {
	return len(pod.Spec.NodeName) != 0
}

func (utMgr *UsageTemplateManager) CacheSize() int {
	counts := 0
	for _, v := range utMgr.NodePodsCache {
		counts += len(v)
	}
	return counts
}

func (utMgr *UsageTemplateManager) GetNodePods(nodeName string) []NamespacedPod {
	utMgr.RWMutex.RLock()
	pods := utMgr.NodePodsCache[nodeName]
	utMgr.RUnlock()
	return pods
}
