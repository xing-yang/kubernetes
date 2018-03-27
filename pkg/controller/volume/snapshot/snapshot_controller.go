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
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"
	//"k8s.io/kubernetes/pkg/util/slice"
	//volumeutil "k8s.io/kubernetes/pkg/volume/util"
)

// Controller is controller that creates/deletes snapshots from PVCs. 
type Controller struct {
	client clientset.Interface

	vsLister       corelisters.VolumeSnapshotLister
	vsListerSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface
}

// NewSnapshotController returns a new *Controller.
func NewSnapshotController(vsInformer coreinformers.VolumeSnapshotInformer, cl clientset.Interface) *Controller {
	e := &Controller{
		client: cl,
		queue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "snapshot"),
	}

	e.vsLister = vsInformer.Lister()
	e.vsListerSynced = vsInformer.Informer().HasSynced
	vsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: e.vsAddedUpdated,
		UpdateFunc: func(old, new interface{}) {
			e.vsAddedUpdated(new)
		},
		//DeleteFunc: e.vsDeletedUpdated,
	})

	return e
}

// Run runs the controller goroutines.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting volume snapshot controller")
	defer glog.Infof("Shutting down volume snapshot controller")

	if !controller.WaitForCacheSync("volume snapshot", stopCh, c.vsListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one vsKey off the queue.  It returns false when it's time to quit.
func (c *Controller) processNextWorkItem() bool {
	vsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(vsKey)

	vsName := vsKey.(string)

	err := c.processVS(vsName)
	if err == nil {
		c.queue.Forget(vsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Volume snapshot %v failed with : %v", vsKey, err))
	c.queue.AddRateLimited(vsKey)

	return true
}

func (c *Controller) processVS(vsName string) error {
	glog.V(4).Infof("Processing volume snapshot %s", vsName)
	startTime := time.Now()
	defer func() {
		glog.V(4).Infof("Finished processing volume snapshot %s (%v)", vsName, time.Now().Sub(startTime))
	}()

	vs, err := c.vsLister.Get(vsName)
	if apierrs.IsNotFound(err) {
		glog.V(4).Infof("Volume snapshot %s not found, ignoring", vsName)
		return nil
	}
	if err != nil {
		return err
	}

	glog.V(4).Infof("Print out volume snapshot info %v", vs)

	/*if isDeletionCandidate(vs) {
		// Volume snapshot should be deleted. Check if it's used and remove finalizer if
		// it's not.
		isUsed := c.isBeingUsed(vs)
		if !isUsed {
			return c.removeFinalizer(vs)
		}
	}

	if needToAddFinalizer(vs) {
		// Volume snapshot is not being deleted -> it should have the finalizer. The
		// finalizer should be added by admission plugin, this is just to add
		// the finalizer to old volume snapshots that were created before the admission
		// plugin was enabled.
		return c.addFinalizer(vs)
	}*/
	return nil
}

/*func (c *Controller) addFinalizer(vs *v1.VolumeSnapshot) error {
	vsClone := pv.DeepCopy()
	vsClone.ObjectMeta.Finalizers = append(vsClone.ObjectMeta.Finalizers, volumeutil.VSProtectionFinalizer)
	_, err := c.client.CoreV1().VolumeSnapshots().Update(vsClone)
	if err != nil {
		glog.V(3).Infof("Error adding protection finalizer to volume snapshot %s: %v", vs.Name)
		return err
	}
	glog.V(3).Infof("Added finalizer to volume snapshot %s", vs.Name)
	return nil
}*/

/*func (c *Controller) removeFinalizer(pv *v1.VolumeSnapshot) error {
	vsClone := vs.DeepCopy()
	vsClone.ObjectMeta.Finalizers = slice.RemoveString(vsClone.ObjectMeta.Finalizers, volumeutil.VSProtectionFinalizer, nil)
	_, err := c.client.CoreV1().VolumeSnapshots().Update(vsClone)
	if err != nil {
		glog.V(3).Infof("Error removing finalizer from volume snapshot %s: %v", vs.Name, err)
		return err
	}
	glog.V(3).Infof("Removed protection finalizer from volume snapshot %s", vs.Name)
	return nil
}*/

/*func (c *Controller) isBeingUsed(pv *v1.VolumeSnapshot) bool {
	// check if volume snapshot is being bound to a PVC by its status
	// the status will be updated by PV controller
	if pv.Status.Phase == v1.VolumeBound {
		// the PV is being used now
		return true
	}

	return false
}*/

// vsAddedUpdated reacts to vs added/updated events
func (c *Controller) vsAddedUpdated(obj interface{}) {
	vs, ok := obj.(*v1.VolumeSnapshot)
	if !ok {
		utilruntime.HandleError(fmt.Errorf("VolumeSnapshot informer returned non-VS object: %#v", obj))
		return
	}
	glog.V(4).Infof("Got event on VolumeSnapshot %s", vs.Metadata.Name)

	//if needToAddFinalizer(vs) || isDeletionCandidate(vs) {
	//	c.queue.Add(vs.Metadata.Name)
	//}
}

/*func isDeletionCandidate(pv *v1.PersistentVolume) bool {
	return pv.ObjectMeta.DeletionTimestamp != nil && slice.ContainsString(pv.ObjectMeta.Finalizers, volumeutil.PVProtectionFinalizer, nil)
}

func needToAddFinalizer(pv *v1.PersistentVolume) bool {
	return pv.ObjectMeta.DeletionTimestamp == nil && !slice.ContainsString(pv.ObjectMeta.Finalizers, volumeutil.PVProtectionFinalizer, nil)
}*/
