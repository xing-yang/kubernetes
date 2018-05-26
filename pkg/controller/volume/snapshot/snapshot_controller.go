/*
Copyright 2018 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package snapshot

import (
	"fmt"
	"net"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1alpha1"

	"k8s.io/apimachinery/pkg/types"
	storageinformers "k8s.io/client-go/informers/storage/v1alpha1"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1alpha1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
	vscache "k8s.io/kubernetes/pkg/controller/volume/snapshot/cache"
	"k8s.io/kubernetes/pkg/controller/volume/snapshot/populator"
	"k8s.io/kubernetes/pkg/controller/volume/snapshot/reconciler"
	"k8s.io/kubernetes/pkg/controller/volume/snapshot/snapshotter"
	"k8s.io/kubernetes/pkg/util/goroutinemap"
	"k8s.io/kubernetes/pkg/util/io"
	"k8s.io/kubernetes/pkg/util/mount"
	"k8s.io/kubernetes/pkg/volume"
)

const (
	reconcilerLoopPeriod time.Duration = 100 * time.Millisecond

	// desiredStateOfWorldPopulatorLoopSleepPeriod is the amount of time the
	// DesiredStateOfWorldPopulator loop waits between successive executions
	desiredStateOfWorldPopulatorLoopSleepPeriod time.Duration = 1 * time.Minute

	// desiredStateOfWorldPopulatorListPodsRetryDuration is the amount of
	// time the DesiredStateOfWorldPopulator loop waits between list snapshots
	// calls.
	desiredStateOfWorldPopulatorListSnapshotsRetryDuration time.Duration = 3 * time.Minute

	defaultSyncDuration time.Duration = 60 * time.Second
)

// Controller is controller that creates/deletes snapshots from PVCs.
type Controller struct {
	client clientset.Interface

	vsLister       storagelisters.VolumeSnapshotLister
	vsListerSynced cache.InformerSynced

	vsdLister       storagelisters.VolumeSnapshotDataLister
	vsdListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// cloud provider used by volume host
	cloud cloudprovider.Interface

	// volumePluginMgr used to initialize and fetch volume plugins
	volumePluginMgr volume.VolumePluginMgr

	// Map of scheduled/running operations.
	runningOperations goroutinemap.GoRoutineMap

	// recorder is used to record events in the API server
	recorder record.EventRecorder

	// desiredStateOfWorld is a data structure containing the desired state of
	// the world according to this controller: i.e. what VolumeSnapshots need
	// the VolumeSnapshotData to be created, what VolumeSnapshotData and their
	// representing "on-disk" snapshots to be removed.
	desiredStateOfWorld vscache.DesiredStateOfWorld

	// actualStateOfWorld is a data structure containing the actual state of
	// the world according to this controller: i.e. which VolumeSnapshots and
	// VolumeSnapshot data exist and to which PV/PVCs are associated.
	actualStateOfWorld vscache.ActualStateOfWorld

	// reconciler is used to run an asynchronous periodic loop to create and delete
	// VolumeSnapshotData for the user created and deleted VolumeSnapshot objects and
	// trigger the actual snapshot creation in the volume backends.
	reconciler reconciler.Reconciler

	// Volume snapshotter is responsible for talking to the backend and creating, removing
	// or promoting the snapshots.
	snapshotter snapshotter.VolumeSnapshotter

	// desiredStateOfWorldPopulator runs an asynchronous periodic loop to
	// populate the current snapshots using snapshotInformer.
	desiredStateOfWorldPopulator populator.DesiredStateOfWorldPopulator
}

// NewSnapshotController returns a new *Controller.
func NewSnapshotController(
	vsInformer storageinformers.VolumeSnapshotInformer,
	vsdInformer storageinformers.VolumeSnapshotDataInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	plugins []volume.VolumePlugin) (*Controller, error) {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "volume_snapshot"})

	ssc := &Controller{
		client:         kubeClient,
		cloud:          cloud,
		recorder:       recorder,
		vsLister:       vsInformer.Lister(),
		vsListerSynced: vsInformer.Informer().HasSynced,
	}

	if err := ssc.volumePluginMgr.InitPlugins(plugins, nil, ssc); err != nil {
		return nil, fmt.Errorf("Could not initialize volume plugins for snapshot Controller : %v", err)
	}

	vsInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ssc.onSnapshotAdd,
			UpdateFunc: ssc.onSnapshotUpdate,
			DeleteFunc: ssc.onSnapshotDelete,
		}, time.Minute*60)

	ssc.desiredStateOfWorld = vscache.NewDesiredStateOfWorld()
	ssc.actualStateOfWorld = vscache.NewActualStateOfWorld()

	ssc.snapshotter = snapshotter.NewVolumeSnapshotter(
		kubeClient,
		ssc.actualStateOfWorld,
		ssc.volumePluginMgr)

	ssc.reconciler = reconciler.NewReconciler(
		reconcilerLoopPeriod,
		defaultSyncDuration,
		false, /* disableReconciliationSync */
		ssc.desiredStateOfWorld,
		ssc.actualStateOfWorld,
		ssc.snapshotter)

	ssc.desiredStateOfWorldPopulator = populator.NewDesiredStateOfWorldPopulator(
		desiredStateOfWorldPopulatorLoopSleepPeriod,
		desiredStateOfWorldPopulatorListSnapshotsRetryDuration,
		ssc.vsLister,
		ssc.desiredStateOfWorld,
	)

	return ssc, nil
}

// Run starts an Snapshot resource controller
func (c *Controller) Run(stopCh <-chan struct{}) {
	glog.Infof("Starting snapshot controller")
	if !controller.WaitForCacheSync("snapshot-controller", stopCh, c.vsListerSynced) {
		return
	}

	go c.reconciler.Run(stopCh)
	go c.desiredStateOfWorldPopulator.Run(stopCh)

}

func (c *Controller) onSnapshotAdd(obj interface{}) {
	// Add snapshot: Add snapshot to DesiredStateOfWorld, then ask snapshotter to create
	// the actual snapshot
	snapshotObj, ok := obj.(*storage.VolumeSnapshot)
	if !ok {
		glog.Warning("expecting type VolumeSnapshot but received type %T", obj)
		return
	}
	snapshot := snapshotObj.DeepCopy()

	glog.Infof("[CONTROLLER] OnAdd %s, Snapshot %#v", snapshot.ObjectMeta.SelfLink, snapshot)
	c.desiredStateOfWorld.AddSnapshot(snapshot)
}

func (c *Controller) onSnapshotUpdate(oldObj, newObj interface{}) {
	oldSnapshot := oldObj.(*storage.VolumeSnapshot)
	newSnapshot := newObj.(*storage.VolumeSnapshot)
	glog.Infof("[CONTROLLER] OnUpdate oldObj: %#v", oldSnapshot.Spec)
	glog.Infof("[CONTROLLER] OnUpdate newObj: %#v", newSnapshot.Spec)
	if oldSnapshot.Spec.SnapshotDataName != newSnapshot.Spec.SnapshotDataName {
		c.desiredStateOfWorld.AddSnapshot(newSnapshot)
	}
}

func (c *Controller) onSnapshotDelete(obj interface{}) {
	deletedSnapshot, ok := obj.(*storage.VolumeSnapshot)
	if !ok {
		// DeletedFinalStateUnkown is an expected data type here
		deletedState, isState := obj.(cache.DeletedFinalStateUnknown)
		if !isState {
			glog.Errorf("Error: unkown type passed as snapshot for deletion: %T", obj)
			return
		}
		deletedSnapshot, ok = deletedState.Obj.(*storage.VolumeSnapshot)
		if !ok {
			glog.Errorf("Error: unkown data type in DeletedState: %T", deletedState.Obj)
			return
		}
	}
	// Delete snapshot: Remove the snapshot from DesiredStateOfWorld, then ask snapshotter to delete
	// the snapshot itself
	snapshot := deletedSnapshot.DeepCopy()
	glog.Infof("[CONTROLLER] OnDelete %s, snapshot name: %s/%s\n", snapshot.ObjectMeta.SelfLink, snapshot.ObjectMeta.Namespace, snapshot.ObjectMeta.Name)
	c.desiredStateOfWorld.DeleteSnapshot(vscache.MakeSnapshotName(snapshot.ObjectMeta.Namespace, snapshot.ObjectMeta.Name))

}

// Implementing VolumeHost interface
func (c *Controller) GetPluginDir(pluginName string) string {
	return ""
}

func (c *Controller) GetVolumeDevicePluginDir(pluginName string) string {
	return ""
}

func (expc *Controller) GetPodsDir() string {
	return ""
}

func (c *Controller) GetPodVolumeDir(podUID types.UID, pluginName string, volumeName string) string {
	return ""
}

func (c *Controller) GetPodVolumeDeviceDir(podUID types.UID, pluginName string) string {
	return ""
}

func (c *Controller) GetPodPluginDir(podUID types.UID, pluginName string) string {
	return ""
}

func (c *Controller) GetKubeClient() clientset.Interface {
	return c.client
}

func (c *Controller) NewWrapperMounter(volName string, spec volume.Spec, pod *v1.Pod, opts volume.VolumeOptions) (volume.Mounter, error) {
	return nil, fmt.Errorf("NewWrapperMounter not supported by expand controller's VolumeHost implementation")
}

func (c *Controller) NewWrapperUnmounter(volName string, spec volume.Spec, podUID types.UID) (volume.Unmounter, error) {
	return nil, fmt.Errorf("NewWrapperUnmounter not supported by expand controller's VolumeHost implementation")
}

func (c *Controller) GetCloudProvider() cloudprovider.Interface {
	return c.cloud
}

func (c *Controller) GetMounter(pluginName string) mount.Interface {
	return nil
}

func (c *Controller) GetExec(pluginName string) mount.Exec {
	return mount.NewOsExec()
}

func (c *Controller) GetWriter() io.Writer {
	return nil
}

func (c *Controller) GetHostName() string {
	return ""
}

func (c *Controller) GetHostIP() (net.IP, error) {
	return nil, fmt.Errorf("GetHostIP not supported by expand controller's VolumeHost implementation")
}

func (c *Controller) GetNodeAllocatable() (v1.ResourceList, error) {
	return v1.ResourceList{}, nil
}

func (c *Controller) GetSecretFunc() func(namespace, name string) (*v1.Secret, error) {
	return func(_, _ string) (*v1.Secret, error) {
		return nil, fmt.Errorf("GetSecret unsupported in Controller")
	}
}

func (c *Controller) GetConfigMapFunc() func(namespace, name string) (*v1.ConfigMap, error) {
	return func(_, _ string) (*v1.ConfigMap, error) {
		return nil, fmt.Errorf("GetConfigMap unsupported in Controller")
	}
}

func (c *Controller) GetNodeLabels() (map[string]string, error) {
	return nil, fmt.Errorf("GetNodeLabels unsupported in Controller")
}

func (c *Controller) GetNodeName() types.NodeName {
	return ""
}

func (c *Controller) GetEventRecorder() record.EventRecorder {
	return c.recorder
}
